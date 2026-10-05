package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type references []string

func (r *references) String() string { return fmt.Sprint([]string(*r)) }
func (r *references) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty reference")
	}
	*r = append(*r, value)
	return nil
}

type savedRequest struct {
	ID             string          `json:"id"`
	Origin         string          `json:"origin"`
	KeyFingerprint string          `json:"keyFingerprint"`
	Kind           string          `json:"kind"`
	Body           json.RawMessage `json:"body"`
	TaskID         string          `json:"taskId,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func (a *app) fingerprint() string {
	h := sha256.Sum256([]byte(a.client.secret))
	return hex.EncodeToString(h[:])
}
func (a *app) requestPath(id string) string { return filepath.Join(a.dir, "requests", id+".json") }
func (a *app) generate(args []string) error {
	if len(args) == 0 || (args[0] != "image" && args[0] != "video" && args[0] != "audio") {
		return &exitError{2, "choose generate image, video or audio"}
	}
	f := a.flags("generate " + args[0])
	model := f.String("model", "", "model ID")
	prompt := f.String("prompt", "", "prompt or script")
	parameters := f.String("parameters", "{}", "JSON object of model parameters")
	operation := f.String("operation", "", "audio operation")
	maxGems := f.Int64("max-gems", -1, "maximum Gem cost")
	async := f.Bool("async", false, "return after submission")
	dryRun := f.Bool("dry-run", false, "print request without writing or calling APIs")
	id := f.String("request-id", "", "persistent request identity (reuse only with identical payload)")
	timeout := f.Duration("timeout", 10*time.Minute, "maximum wait")
	output := f.String("output", "", "download outputs to this directory")
	var refs references
	f.Var(&refs, "reference", "owned asset ID (repeatable)")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *model == "" || *prompt == "" || *maxGems < -1 || *maxGems > 1_000_000_000 || *timeout <= 0 || len(refs) > 10 {
		return &exitError{2, "invalid generation options; model and prompt are required"}
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(*parameters), &params); err != nil || params == nil {
		return &exitError{2, "parameters must be a JSON object"}
	}
	if args[0] == "audio" && (*operation != "speech" && *operation != "music" && *operation != "voice_clone") {
		return &exitError{2, "audio needs --operation speech, music or voice_clone"}
	}
	if args[0] != "audio" && *operation != "" {
		return &exitError{2, "operation applies to audio"}
	}
	if len(refs) == 0 {
		refs = references{}
	}
	body := map[string]any{"model": *model, "prompt": *prompt, "parameters": params, "referenceAssetIds": refs}
	if *operation != "" {
		body["operation"] = *operation
	}
	if *maxGems >= 0 {
		body["maxGems"] = *maxGems
	}
	if *dryRun {
		return a.emit(map[string]any{"method": "POST", "url": a.origin + "/api/generate/" + args[0], "body": body, "cost": "server-calculated; no request sent"})
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	if *id == "" {
		*id = randomURLToken()
	}
	if !requestIDPattern.MatchString(*id) {
		return &exitError{2, "invalid request ID"}
	}
	b, _ := json.Marshal(body)
	record := savedRequest{*id, a.origin, a.fingerprint(), args[0], b, "", time.Now().UTC().Format(time.RFC3339)}
	if old, err := a.readRequest(*id); err == nil {
		if old.Origin != record.Origin || old.KeyFingerprint != record.KeyFingerprint || old.Kind != record.Kind || string(old.Body) != string(record.Body) {
			return fmt.Errorf("request identity already belongs to another payload, origin or credential")
		}
		record = old
	} else if !os.IsNotExist(err) {
		return err
	} else if err := a.createRequest(record); err != nil {
		return err
	}
	return a.submit(record, *async, *timeout, *output)
}
func (a *app) createRequest(record savedRequest) error {
	// O_EXCL prevents two callers from silently replacing one durable identity.
	path := a.requestPath(record.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(record)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func (a *app) readRequest(id string) (savedRequest, error) {
	var r savedRequest
	if !requestIDPattern.MatchString(id) {
		return r, &exitError{2, "invalid request ID"}
	}
	b, err := os.ReadFile(a.requestPath(id))
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("invalid saved request")
	}
	if r.ID != id || !json.Valid(r.Body) || (r.Kind != "image" && r.Kind != "video" && r.Kind != "audio") {
		return r, fmt.Errorf("invalid saved request")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, r.Body); err != nil {
		return r, err
	}
	r.Body = compact.Bytes()
	return r, nil
}
func (a *app) submit(record savedRequest, async bool, timeout time.Duration, output string) error {
	fmt.Fprintf(a.errOut, "Request identity: %s\nRecover with: letsgen --origin %s requests retry %s\n", record.ID, record.Origin, record.ID)
	var current task
	if record.TaskID != "" {
		var err error
		current, err = a.getTask(record.TaskID)
		if err != nil {
			return err
		}
	} else {
		var response struct {
			Task task `json:"task"`
		}
		if err := a.client.json(a.ctx, "POST", "/api/generate/"+record.Kind, record.Body, map[string]string{"Idempotency-Key": record.ID}, &response); err != nil {
			return err
		}
		current = response.Task
		if current.ID == "" {
			return fmt.Errorf("API returned no task; retry the saved request")
		}
		record.TaskID = current.ID
		if err := writeJSON(a.requestPath(record.ID), record); err != nil {
			return fmt.Errorf("task %s submitted; failed to save its ID: %w", current.ID, err)
		}
	}
	if async {
		return a.emit(map[string]any{"task": current, "requestId": record.ID})
	}
	finished, err := a.waitTask(current, timeout)
	if err != nil {
		_ = a.emit(map[string]any{"task": finished, "requestId": record.ID})
		return err
	}
	if output != "" {
		return a.downloadTask(finished, output)
	}
	return a.emit(map[string]any{"task": finished, "requestId": record.ID})
}
func (a *app) requests(args []string) error {
	if len(args) == 1 && args[0] == "list" {
		files, err := filepath.Glob(filepath.Join(a.dir, "requests", "*.json"))
		if err != nil {
			return err
		}
		items := []map[string]any{}
		for _, p := range files {
			id := filepath.Base(p)
			id = id[:len(id)-5]
			r, err := a.readRequest(id)
			if err != nil {
				return err
			}
			items = append(items, map[string]any{"id": r.ID, "origin": r.Origin, "kind": r.Kind, "taskId": r.TaskID, "createdAt": r.CreatedAt})
		}
		return a.emit(map[string]any{"requests": items})
	}
	if len(args) < 2 || args[0] != "retry" {
		return &exitError{2, "usage: requests list or requests retry ID [--async] [--timeout 10m]"}
	}
	f := a.flags("requests retry")
	async := f.Bool("async", false, "return after submission")
	timeout := f.Duration("timeout", 10*time.Minute, "maximum wait")
	output := f.String("output", "", "download directory")
	if err := f.Parse(args[2:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *timeout <= 0 {
		return &exitError{2, "invalid retry options"}
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	r, err := a.readRequest(args[1])
	if err != nil {
		return err
	}
	if r.Origin != a.origin || r.KeyFingerprint != a.fingerprint() {
		return fmt.Errorf("retry requires the original API origin and credential")
	}
	return a.submit(r, *async, *timeout, *output)
}
