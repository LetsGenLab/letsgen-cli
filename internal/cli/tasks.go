package cli

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type task struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	QuotedGems   int          `json:"quotedGems"`
	ChargedGems  int          `json:"chargedGems"`
	ReleasedGems int          `json:"releasedGems"`
	Error        any          `json:"error"`
	Outputs      []taskOutput `json:"outputs"`
}
type taskOutput struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	Asset  *struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		MIME string `json:"mimeType"`
		URL  string `json:"url"`
	} `json:"asset"`
}

func terminal(t task) bool {
	switch t.Status {
	case "succeeded", "partial", "failed", "cancelled":
		return true
	}
	return false
}
func (a *app) getTask(id string) (task, error) {
	var response struct {
		Task task `json:"task"`
	}
	if id == "" {
		return response.Task, &exitError{2, "task ID required"}
	}
	err := a.client.json(a.ctx, "GET", "/api/generate/tasks/"+url.PathEscape(id), nil, nil, &response)
	if err == nil && (response.Task.ID != id || response.Task.Status == "") {
		err = fmt.Errorf("invalid task response")
	}
	return response.Task, err
}
func (a *app) waitTask(t task, timeout time.Duration) (task, error) {
	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()
	for !terminal(t) {
		fmt.Fprintf(a.errOut, "Task %s: %s\n", t.ID, t.Status)
		select {
		case <-ctx.Done():
			return t, &exitError{4, fmt.Sprintf("wait interrupted; resume with letsgen tasks wait %s", t.ID)}
		case <-time.After(2 * time.Second):
		}
		var response struct {
			Task task `json:"task"`
		}
		if err := a.client.json(ctx, "GET", "/api/generate/tasks/"+url.PathEscape(t.ID), nil, nil, &response); err != nil {
			if ctx.Err() != nil {
				return t, &exitError{4, "wait timed out; the task continues on the server"}
			}
			return t, err
		}
		if response.Task.ID != t.ID || response.Task.Status == "" {
			return t, fmt.Errorf("invalid task response")
		}
		t = response.Task
	}
	if t.Status == "failed" || t.Status == "cancelled" {
		return t, &exitError{5, fmt.Sprintf("task %s %s", t.ID, t.Status)}
	}
	return t, nil
}
func (a *app) tasks(args []string) error {
	if len(args) < 2 {
		return &exitError{2, "usage: tasks get|wait|download ID"}
	}
	f := a.flags("tasks " + args[0])
	timeout := f.Duration("timeout", 10*time.Minute, "maximum wait")
	output := f.String("output", "./out", "download directory")
	if err := f.Parse(args[2:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *timeout <= 0 {
		return &exitError{2, "invalid task options"}
	}
	if args[0] != "get" && args[0] != "wait" && args[0] != "download" {
		return &exitError{2, "unknown task command"}
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	t, err := a.getTask(args[1])
	if err != nil {
		return err
	}
	if args[0] == "wait" {
		t, err = a.waitTask(t, *timeout)
		if err != nil {
			_ = a.emit(map[string]any{"task": t})
			return err
		}
	}
	if args[0] == "download" {
		return a.downloadTask(t, *output)
	}
	return a.emit(map[string]any{"task": t})
}
func (a *app) downloadTask(t task, dir string) error {
	if !terminal(t) {
		return fmt.Errorf("task is still processing; wait before downloading")
	}
	if !requestIDPattern.MatchString(t.ID) {
		return fmt.Errorf("unsupported task ID for download")
	}
	paths := []string{}
	for _, o := range t.Outputs {
		if o.Status != "succeeded" || o.Asset == nil || o.Asset.URL == "" {
			continue
		}
		ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif", "video/mp4": ".mp4", "video/webm": ".webm", "audio/mpeg": ".mp3", "audio/mp3": ".mp3", "audio/wav": ".wav", "audio/x-wav": ".wav", "audio/ogg": ".ogg", "audio/flac": ".flac"}[o.Asset.MIME]
		if ext == "" {
			ext = ".bin"
		}
		path := filepath.Join(dir, fmt.Sprintf("%s-%d%s", t.ID, o.Index, ext))
		if err := download(a.ctx, o.Asset.URL, path); err != nil {
			return err
		}
		abs, _ := filepath.Abs(path)
		paths = append(paths, abs)
	}
	if len(paths) == 0 {
		return &exitError{5, "task has no downloadable successful outputs"}
	}
	return a.emit(map[string]any{"taskId": t.ID, "files": paths, "status": t.Status})
}
func download(ctx context.Context, value, path string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("unsupported output URL")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists or cannot be accessed: %s", path)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	// Never send API credentials to media hosts, including redirects.
	transport := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := transport.Do(req)
	if err != nil {
		return fmt.Errorf("output download failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("output download failed (HTTP %d); refresh task URLs and retry", res.StatusCode)
	}
	contentType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if contentType == "text/html" {
		return fmt.Errorf("output returned a page instead of media")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".letsgen-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	const limit = 512 * 1024 * 1024
	count, err := io.Copy(temp, io.LimitReader(res.Body, limit+1))
	if err != nil {
		return fmt.Errorf("output download interrupted")
	}
	if count == 0 || count > limit {
		return fmt.Errorf("output size must be between 1 byte and 512 MB")
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// Hard-link atomically publishes a complete file without overwriting existing media.
	if err := os.Link(temp.Name(), path); err != nil {
		return fmt.Errorf("could not save output without overwriting: %w", err)
	}
	return nil
}
