package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/term"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var apiKeyPattern = regexp.MustCompile(`^lgk_[a-f0-9]{64}$`)

func (a *app) auth(args []string) error {
	if len(args) == 0 {
		return &exitError{2, "choose auth login, status or logout"}
	}
	switch args[0] {
	case "login":
		f := a.flags("auth login")
		key := f.Bool("api-key", false, "read API key from hidden terminal prompt (or stdin)")
		noBrowser := f.Bool("no-browser", false, "print URL instead of opening browser")
		cap := f.Int("monthly-gem-cap", 100, "requested monthly cap")
		readOnly := f.Bool("read-only", false, "request read-only access")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 || *cap < 0 || *cap > 1_000_000_000 {
			return &exitError{2, "invalid login options"}
		}
		if *key {
			return a.keyLogin()
		}
		return a.browserLogin(*noBrowser, *cap, *readOnly)
	case "status":
		if len(args) != 1 {
			return &exitError{2, "auth status takes no arguments"}
		}
		if err := a.needAuth(); err != nil {
			return err
		}
		var result any
		if err := a.client.json(a.ctx, "GET", "/api/cli/token", nil, nil, &result); err != nil {
			return err
		}
		return a.emit(result)
	case "logout":
		if len(args) != 1 {
			return &exitError{2, "auth logout takes no arguments"}
		}
		if os.Getenv("LETSGEN_API_KEY") != "" {
			return &exitError{2, "unset LETSGEN_API_KEY before revoking saved credentials; revoke environment keys in Settings → API keys"}
		}
		saved := a.cfg.Credentials[a.origin]
		if saved.Secret != "" {
			err := a.client.json(a.ctx, "DELETE", "/api/cli/token", nil, nil, nil)
			var api *apiError
			if err != nil && !(errors.As(err, &api) && api.Status == 401) {
				return err
			}
		}
		delete(a.cfg.Credentials, a.origin)
		if err := writeJSON(filepath.Join(a.dir, "config.json"), a.cfg); err != nil {
			return err
		}
		return a.emit(map[string]bool{"loggedOut": true})
	default:
		return &exitError{2, "unknown auth command"}
	}
}
func (a *app) saveCredential(value credential) error {
	a.cfg.Credentials[a.origin] = value
	if err := writeJSON(filepath.Join(a.dir, "config.json"), a.cfg); err != nil {
		return err
	}
	return a.emit(map[string]any{"authenticated": true, "origin": a.origin, "keyId": value.KeyID})
}
func (a *app) keyLogin() error {
	fmt.Fprint(a.errOut, "API key: ")
	var secret string
	if file, ok := a.in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		b, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(a.errOut)
		if err != nil {
			return err
		}
		secret = strings.TrimSpace(string(b))
	} else {
		line, err := bufio.NewReader(a.in).ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("read API key from stdin")
		}
		secret = strings.TrimSpace(line)
	}
	if !apiKeyPattern.MatchString(secret) {
		return &exitError{2, "invalid personal API key format"}
	}
	// Validate using the established model route, including servers before CLI login support.
	check := newClient(a.origin, secret)
	if err := check.json(a.ctx, "GET", "/api/generate/models", nil, nil, nil); err != nil {
		return err
	}
	return a.saveCredential(credential{Secret: secret})
}
func randomURLToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

type callbackResult struct {
	code string
	err  error
}

func callbackHandler(state string, result chan<- callbackResult) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.Method != "GET" || r.URL.Path != "/callback" {
			http.Error(w, "Not found", 404)
			return
		}
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid sign-in state", 400)
			return
		}
		var value callbackResult
		if r.URL.Query().Get("error") != "" {
			value.err = fmt.Errorf("sign-in was cancelled")
		} else {
			value.code = r.URL.Query().Get("code")
			if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(value.code) {
				http.Error(w, "Invalid authorization", 400)
				return
			}
		}
		select {
		case result <- value:
			fmt.Fprintln(w, "Return to your terminal to complete sign-in. You may close this tab.")
		default:
			http.Error(w, "Already received", 409)
		}
	})
}
func (a *app) browserLogin(noBrowser bool, cap int, readOnly bool) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	state, verifier := randomURLToken(), randomURLToken()
	hash := sha256.Sum256([]byte(verifier))
	callback := "http://" + listener.Addr().String() + "/callback"
	result := make(chan callbackResult, 1)
	server := &http.Server{Handler: callbackHandler(state, result), ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	u, _ := url.Parse(a.origin + "/cli/connect")
	q := u.Query()
	q.Set("redirect_uri", callback)
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(hash[:]))
	q.Set("code_challenge_method", "S256")
	q.Set("monthly_gem_cap", fmt.Sprint(cap))
	q.Set("read_only", fmt.Sprint(readOnly))
	u.RawQuery = q.Encode()
	fmt.Fprintf(a.errOut, "Approve CLI access in your browser:\n%s\n", u.String())
	if !noBrowser {
		if err := openBrowser(a.ctx, u.String()); err != nil {
			fmt.Fprintln(a.errOut, "Open the URL above to continue.")
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	select {
	case <-ctx.Done():
		return &exitError{4, "browser login timed out or was interrupted"}
	case value := <-result:
		if value.err != nil {
			return value.err
		}
		var response struct {
			Secret string `json:"secret"`
			Key    struct {
				ID string `json:"id"`
			} `json:"key"`
		}
		// Exchange is intentionally single-attempt: the authorization code is single-use.
		if err := newClient(a.origin, "").json(ctx, "POST", "/api/cli/token", map[string]string{"code": value.code, "redirectUri": callback, "codeVerifier": verifier}, nil, &response); err != nil {
			return err
		}
		if !apiKeyPattern.MatchString(response.Secret) {
			return fmt.Errorf("invalid sign-in response")
		}
		return a.saveCredential(credential{response.Secret, response.Key.ID})
	}
}
func openBrowser(ctx context.Context, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "open", value).Run()
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", value).Run()
	default:
		return exec.CommandContext(ctx, "xdg-open", value).Run()
	}
}
