package cli

import (
 "encoding/json"
 "errors"
 "fmt"
 "net/url"
 "os"
 "path/filepath"
 "strings"
)

const defaultOrigin = "https://letsgen.app"

type credential struct {
 Secret string `json:"secret"`
 KeyID string `json:"keyId,omitempty"`
}
type config struct { Credentials map[string]credential `json:"credentials"` }

func configDir() (string, error) {
 if p := os.Getenv("LETSGEN_CONFIG_DIR"); p != "" { return filepath.Abs(p) }
 p, err := os.UserConfigDir()
 return filepath.Join(p, "letsgen"), err
}
func loadConfig(dir string) (config, error) {
 c := config{Credentials: map[string]credential{}}
 b, err := os.ReadFile(filepath.Join(dir, "config.json"))
 if errors.Is(err, os.ErrNotExist) { return c, nil }
 if err != nil { return c, err }
 if err = json.Unmarshal(b, &c); err != nil { return c, fmt.Errorf("invalid credential file") }
 if c.Credentials == nil { c.Credentials = map[string]credential{} }
 return c, nil
}
func writeJSON(path string, value any) error {
 if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return err }
 b, err := json.MarshalIndent(value, "", "  ")
 if err != nil { return err }
 f, err := os.CreateTemp(filepath.Dir(path), ".letsgen-*")
 if err != nil { return err }
 name := f.Name()
 defer os.Remove(name)
 if err = f.Chmod(0600); err == nil { _, err = f.Write(append(b, '\n')) }
 closeErr := f.Close()
 if err != nil { return err }
 if closeErr != nil { return closeErr }
 return os.Rename(name, path)
}
func normalizeOrigin(value string) (string, error) {
 u, err := url.Parse(value)
 if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") { return "", fmt.Errorf("choose an API origin without a path, credentials or query") }
 local := u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"
 if u.Scheme != "https" && !(u.Scheme == "http" && local) { return "", fmt.Errorf("API origin must use HTTPS (HTTP is allowed only on loopback)") }
 return strings.TrimRight(u.String(), "/"), nil
}
