package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type originTransport func(*http.Request) (*http.Response, error)

func (f originTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultRequestsUseProductionAndItsCredential(t *testing.T) {
	dir := isolated(t)
	t.Setenv("LETSGEN_API_ORIGIN", "")
	t.Setenv("LETSGEN_API_KEY", "")
	err := writeJSON(filepath.Join(dir, "config.json"), config{Credentials: map[string]credential{
		"https://letsgen.app":      {Secret: testKey},
		"https://api.example.test": {Secret: "other-environment-credential"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = originTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://letsgen.app/api/cli/token" {
			t.Errorf("unexpected default destination: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer "+testKey {
			t.Error("default request used another environment's credential")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"authenticated":true}`))}, nil
	})
	var out, stderr bytes.Buffer
	code := Main(context.Background(), []string{"--json", "auth", "status"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 0 || calls != 1 {
		t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr.String())
	}
}
