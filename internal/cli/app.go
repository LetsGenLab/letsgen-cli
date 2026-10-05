package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type app struct {
	ctx         context.Context
	in          io.Reader
	out, errOut io.Writer
	dir, origin string
	cfg         config
	client      *client
	jsonOutput  bool
}

const help = `Lets Gen CLI — standalone API client (preview)

letsgen [--origin URL] [--json] COMMAND

  auth login [--api-key] [--no-browser] [--monthly-gem-cap 100] [--read-only]
  auth status | auth logout
  models list [--kind image|video|audio|text] | models inspect ID
  generate image|video|audio --model ID --prompt TEXT [--parameters JSON]
           [--reference ID] [--max-gems N] [--operation speech|music|voice_clone]
           [--async] [--dry-run] [--request-id ID] [--timeout 10m] [--output DIR]
  requests list | requests retry ID [--async] [--timeout 10m]
  assets upload FILE [--mime TYPE]
  voices list [--scope mine|explore] [--language CODE] [--page N]
  tasks get ID | tasks wait ID [--timeout 10m] | tasks download ID [--output DIR]
  skills list | skills show | skills install --target codex|claude [--path DIR]
  version

LETSGEN_API_KEY overrides saved credentials. Never pass a key as an argument.
LETSGEN_API_ORIGIN defaults to https://letsgen.app in this preview.
Global --json and --origin are accepted anywhere; diagnostics use stderr.
Exit codes: 0 success, 1 failure, 2 usage, 3 auth, 4 timeout, 5 task failure.
`

type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string { return e.message }
func Main(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, version string) int {
	a := &app{ctx: ctx, in: in, out: out, errOut: errOut}
	origin := os.Getenv("LETSGEN_API_ORIGIN")
	if origin == "" {
		origin = defaultOrigin
	}
	filtered := []string{}
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			a.jsonOutput = true
		case args[i] == "--origin":
			i++
			if i >= len(args) {
				fmt.Fprintln(errOut, "--origin requires a value")
				return 2
			}
			origin = args[i]
		case strings.HasPrefix(args[i], "--origin="):
			origin = strings.TrimPrefix(args[i], "--origin=")
		default:
			filtered = append(filtered, args[i])
		}
	}
	if len(filtered) == 0 || filtered[0] == "help" || filtered[0] == "--help" || filtered[0] == "-h" {
		fmt.Fprint(out, help)
		return 0
	}
	if filtered[0] == "version" {
		fmt.Fprintln(out, version)
		return 0
	}
	var err error
	a.origin, err = normalizeOrigin(origin)
	if err == nil {
		a.dir, err = configDir()
	}
	if err == nil {
		a.cfg, err = loadConfig(a.dir)
	}
	if err == nil {
		secret := os.Getenv("LETSGEN_API_KEY")
		if secret == "" {
			secret = a.cfg.Credentials[a.origin].Secret
		}
		a.client = newClient(a.origin, secret)
		err = a.run(filtered)
	}
	if err == nil {
		return 0
	}
	code := 1
	var ex *exitError
	var api *apiError
	if errors.As(err, &ex) {
		code = ex.code
	}
	if errors.As(err, &api) && (api.Status == 401 || api.Status == 403) {
		code = 3
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if a.jsonOutput {
		_ = json.NewEncoder(errOut).Encode(map[string]any{"error": err.Error(), "exitCode": code})
	} else {
		fmt.Fprintln(errOut, err)
	}
	return code
}
func (a *app) run(args []string) error {
	switch args[0] {
	case "auth":
		return a.auth(args[1:])
	case "models":
		return a.models(args[1:])
	case "generate":
		return a.generate(args[1:])
	case "requests":
		return a.requests(args[1:])
	case "assets":
		return a.assets(args[1:])
	case "voices":
		return a.voices(args[1:])
	case "tasks":
		return a.tasks(args[1:])
	case "skills":
		return a.skills(args[1:])
	default:
		return &exitError{2, "unknown command; run letsgen help"}
	}
}
func (a *app) flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(a.errOut)
	return f
}
func (a *app) emit(v any) error {
	e := json.NewEncoder(a.out)
	if !a.jsonOutput {
		e.SetIndent("", "  ")
	}
	return e.Encode(v)
}
func (a *app) needAuth() error {
	if a.client.secret == "" {
		return &exitError{3, "sign in with letsgen auth login or set LETSGEN_API_KEY"}
	}
	return nil
}
