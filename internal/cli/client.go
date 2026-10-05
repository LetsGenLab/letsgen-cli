package cli

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "time"
)

type apiError struct { Status int; Code string }
func (e *apiError) Error() string { return fmt.Sprintf("API request failed (HTTP %d, %s)", e.Status, e.Code) }

type client struct { origin, secret string; http *http.Client }
func newClient(origin, secret string) *client {
 return &client{origin, secret, &http.Client{Timeout: 60*time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *client) request(ctx context.Context, method, path string, body io.Reader, headers map[string]string, out any) error {
 req, err := http.NewRequestWithContext(ctx, method, c.origin+path, body)
 if err != nil { return err }
 if c.secret != "" { req.Header.Set("Authorization", "Bearer "+c.secret) }
 req.Header.Set("User-Agent", "letsgen-cli/0.1")
 req.Header.Set("Accept", "application/json")
 for key, value := range headers { req.Header.Set(key, value) }
 res, err := c.http.Do(req)
 if err != nil { return fmt.Errorf("request interrupted; reuse the saved request identity if submitting") }
 defer res.Body.Close()
 b, err := io.ReadAll(io.LimitReader(res.Body, 8*1024*1024+1))
 if err != nil || len(b) > 8*1024*1024 { return fmt.Errorf("unreadable API response; preserve the request identity") }
 if res.StatusCode < 200 || res.StatusCode >= 300 {
  var payload struct { Code string `json:"code"` }
  _ = json.Unmarshal(b, &payload)
  code := payload.Code
  for _, ch := range code { if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') { code = ""; break } }
  if code == "" || len(code) > 60 { code = "REQUEST_FAILED" }
  return &apiError{res.StatusCode, code}
 }
 if out != nil { if err := json.Unmarshal(b, out); err != nil { return fmt.Errorf("invalid API JSON; preserve the request identity") } }
 return nil
}
func (c *client) json(ctx context.Context, method, path string, body any, headers map[string]string, out any) error {
 var reader io.Reader
 if body != nil {
  b, err := json.Marshal(body); if err != nil { return err }; reader = bytes.NewReader(b)
 }
 h := map[string]string{"Content-Type":"application/json"}
 for k, v := range headers { h[k] = v }
 return c.request(ctx, method, path, reader, h, out)
}
