package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKey = "lgk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func runCLI(t *testing.T, origin string, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := Main(context.Background(), append([]string{"--origin", origin, "--json"}, args...), strings.NewReader(""), &out, &stderr, "test")
	return code, out.String(), stderr.String()
}
func isolated(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LETSGEN_CONFIG_DIR", dir)
	t.Setenv("LETSGEN_API_KEY", testKey)
	return dir
}
func TestDryRunNeverUsesNetworkOrPersistsIdentity(t *testing.T) {
	dir := isolated(t)
	t.Setenv("LETSGEN_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("dry run touched network") }))
	defer srv.Close()
	code, out, stderr := runCLI(t, srv.URL, "generate", "image", "--model", "dynamic", "--prompt", "hello", "--max-gems", "10", "--dry-run")
	if code != 0 || !json.Valid([]byte(out)) || stderr != "" {
		t.Fatalf("code=%d out=%s stderr=%s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "requests")); !os.IsNotExist(err) {
		t.Fatal("dry-run saved identity")
	}
}
func TestUnknownSubmissionPersistsBeforePOSTAndRetryReusesKey(t *testing.T) {
	dir := isolated(t)
	calls := 0
	var firstBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			calls++
			if r.Header.Get("Idempotency-Key") != "stable" {
				t.Error("lost identity")
			}
			data, err := os.ReadFile(filepath.Join(dir, "requests", "stable.json"))
			if err != nil {
				t.Error("request not persisted before submission")
			}
			var record savedRequest
			_ = json.Unmarshal(data, &record)
			if calls == 1 {
				firstBody = string(record.Body)
				w.WriteHeader(503)
				fmt.Fprint(w, `{"code":"REQUEST_UNCERTAIN","error":"sensitive upstream text"}`)
				return
			}
			if string(record.Body) != firstBody {
				t.Error("payload changed on retry")
			}
			fmt.Fprint(w, `{"task":{"id":"run1","status":"queued"}}`)
		} else {
			fmt.Fprint(w, `{"task":{"id":"run1","status":"succeeded","outputs":[]}}`)
		}
	}))
	defer srv.Close()
	code, out, stderr := runCLI(t, srv.URL, "generate", "image", "--model", "one", "--prompt", "original", "--request-id", "stable", "--async")
	if code != 1 || out != "" || strings.Contains(stderr, "sensitive") {
		t.Fatalf("bad uncertain response %d %s %s", code, out, stderr)
	}
	code, out, stderr = runCLI(t, srv.URL, "requests", "retry", "stable", "--async")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("retry %d %s", code, stderr)
	}
	// Saved TaskID bypasses POST; recovery cannot generate again.
	code, _, stderr = runCLI(t, srv.URL, "requests", "retry", "stable", "--async")
	if code != 0 || calls != 2 {
		t.Fatalf("resubmitted known task %d %s", code, stderr)
	}
	code, _, _ = runCLI(t, srv.URL, "generate", "image", "--model", "one", "--prompt", "changed", "--request-id", "stable", "--async")
	if code != 1 || calls != 2 {
		t.Fatal("changed payload reused identity")
	}
	t.Setenv("LETSGEN_API_KEY", strings.Replace(testKey, "a", "b", 1))
	code, _, _ = runCLI(t, srv.URL, "requests", "retry", "stable", "--async")
	if code != 1 || calls != 2 {
		t.Fatal("another key retried identity")
	}
}
func TestTimeoutReturnsTaskForRecoveryAndFailureExitCode(t *testing.T) {
	isolated(t)
	status := "processing"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"task":{"id":"job","status":%q}}`, status)
	}))
	defer srv.Close()
	code, out, _ := runCLI(t, srv.URL, "tasks", "wait", "job", "--timeout", "5ms")
	if code != 4 || !strings.Contains(out, `"id":"job"`) || calls != 1 {
		t.Fatalf("timeout %d %s calls=%d", code, out, calls)
	}
	status = "failed"
	code, out, _ = runCLI(t, srv.URL, "tasks", "wait", "job")
	if code != 5 || !json.Valid([]byte(out)) {
		t.Fatalf("failure %d %s", code, out)
	}
}
func TestCredentialOriginIsolationAndPermissions(t *testing.T) {
	dir := isolated(t)
	t.Setenv("LETSGEN_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testKey {
			t.Error("missing bearer")
		}
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer srv.Close()
	var out, stderr bytes.Buffer
	code := Main(context.Background(), []string{"--origin", srv.URL, "auth", "login", "--api-key"}, strings.NewReader(testKey+"\n"), &out, &stderr, "test")
	if code != 0 || strings.Contains(out.String()+stderr.String(), testKey) {
		t.Fatal("credential leak or login failed")
	}
	info, _ := os.Stat(filepath.Join(dir, "config.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("credential crossed origins") }))
	defer other.Close()
	code, _, _ = runCLI(t, other.URL, "models", "list")
	if code != 3 {
		t.Fatalf("other origin code=%d", code)
	}
}
func TestCallbackRejectsWrongStateAndDoesNotConsumeResult(t *testing.T) {
	result := make(chan callbackResult, 1)
	handler := callbackHandler("expected", result)
	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, httptest.NewRequest("GET", "http://127.0.0.1/callback?state=bad&code="+strings.Repeat("a", 64), nil))
	if bad.Code != 400 || len(result) != 0 {
		t.Fatal("bad state consumed")
	}
	good := httptest.NewRecorder()
	handler.ServeHTTP(good, httptest.NewRequest("GET", "http://127.0.0.1/callback?state=expected&code="+strings.Repeat("b", 64), nil))
	if good.Code != 200 || len(result) != 1 {
		t.Fatal("callback rejected")
	}
	if strings.Contains(good.Body.String(), strings.Repeat("b", 64)) {
		t.Fatal("callback reflects secret code")
	}
}
func TestNoAutomaticRedirectOrSensitiveError(t *testing.T) {
	isolated(t)
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("followed API redirect") }))
	defer destination.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", destination.URL)
		w.WriteHeader(302)
		fmt.Fprint(w, `{"error":"`+testKey+`"}`)
	}))
	defer srv.Close()
	code, out, err := runCLI(t, srv.URL, "models", "list")
	if code != 1 || out != "" || strings.Contains(err, testKey) {
		t.Fatal("redirect/error isolation failed")
	}
}
func TestModelFiltersAndInspectRemainDynamic(t *testing.T) {
	isolated(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"id":"new","kind":"image","capabilities":{"arbitrary":[1,2]}},{"id":"video","kind":"video"}]}`)
	}))
	defer srv.Close()
	code, out, _ := runCLI(t, srv.URL, "models", "list", "--kind", "image")
	if code != 0 || strings.Contains(out, `"id":"video"`) {
		t.Fatal("filter failed")
	}
	code, out, _ = runCLI(t, srv.URL, "models", "inspect", "new")
	if code != 0 || !strings.Contains(out, "arbitrary") {
		t.Fatal("dynamic capabilities lost")
	}
}
func TestUploadUsesOwnedRawMediaAndVoicesAreReadOnly(t *testing.T) {
	dir := isolated(t)
	file := filepath.Join(dir, "ref.png")
	_ = os.WriteFile(file, []byte("fake-image"), 0600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate/assets":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "image/png" || r.Header.Get("X-Filename") != "ref.png" {
				t.Error("wrong upload contract")
			}
			fmt.Fprint(w, `{"asset":{"id":"owned"}}`)
		case "/api/generate/voices":
			if r.Method != "GET" || r.URL.Query().Get("scope") != "explore" {
				t.Error("wrong voices contract")
			}
			fmt.Fprint(w, `{"voices":[]}`)
		default:
			t.Error("unexpected endpoint")
		}
	}))
	defer srv.Close()
	code, _, stderr := runCLI(t, srv.URL, "assets", "upload", file)
	if code != 0 {
		t.Fatal(stderr)
	}
	code, _, stderr = runCLI(t, srv.URL, "voices", "list", "--scope", "explore")
	if code != 0 {
		t.Fatal(stderr)
	}
}
func TestSkillInstallDoesNotOverwrite(t *testing.T) {
	dir := isolated(t)
	code, _, err := runCLI(t, defaultOrigin, "skills", "install", "--target", "codex", "--path", dir)
	if code != 0 {
		t.Fatal(err)
	}
	code, _, _ = runCLI(t, defaultOrigin, "skills", "install", "--target", "codex", "--path", dir)
	if code != 1 {
		t.Fatal("overwrote skill")
	}
}
func TestDownloadRejectsInsecureURLAndExistingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.png")
	if download(context.Background(), "http://example.com/media", path) == nil {
		t.Fatal("insecure download accepted")
	}
	_ = os.WriteFile(path, []byte("existing"), 0600)
	if download(context.Background(), "https://example.com/media", path) == nil {
		t.Fatal("overwrite accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "existing" {
		t.Fatal("existing file changed")
	}
}
