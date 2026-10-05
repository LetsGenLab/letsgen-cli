package cli

import (
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *app) models(args []string) error {
	if len(args) == 0 {
		return &exitError{2, "choose models list or inspect"}
	}
	kind := ""
	if args[0] == "list" {
		f := a.flags("models list")
		f.StringVar(&kind, "kind", "", "media kind")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return &exitError{2, "unexpected argument"}
		}
	} else if args[0] != "inspect" || len(args) != 2 {
		return &exitError{2, "usage: models list or models inspect ID"}
	}
	if kind != "" && kind != "image" && kind != "video" && kind != "audio" && kind != "text" {
		return &exitError{2, "unsupported model kind"}
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	var response struct {
		Models           []map[string]any `json:"models"`
		AccountingPeriod string           `json:"accountingPeriod"`
	}
	if err := a.client.json(a.ctx, "GET", "/api/generate/models", nil, nil, &response); err != nil {
		return err
	}
	selected := []map[string]any{}
	for _, m := range response.Models {
		if args[0] == "inspect" {
			if m["id"] == args[1] {
				return a.emit(m)
			}
		} else if kind == "" || m["kind"] == kind {
			selected = append(selected, m)
		}
	}
	if args[0] == "inspect" {
		return fmt.Errorf("model not found")
	}
	return a.emit(map[string]any{"models": selected, "accountingPeriod": response.AccountingPeriod})
}
func (a *app) assets(args []string) error {
	if len(args) < 2 || args[0] != "upload" {
		return &exitError{2, "usage: assets upload FILE [--mime TYPE]"}
	}
	f := a.flags("assets upload")
	contentType := f.String("mime", "", "media MIME type")
	if err := f.Parse(args[2:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return &exitError{2, "unexpected argument"}
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	file, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 30*1024*1024 {
		return &exitError{2, "choose a regular media file of 1 byte to 30 MB"}
	}
	data := make([]byte, info.Size())
	if _, err := file.ReadAt(data, 0); err != nil {
		return err
	}
	if *contentType == "" {
		*contentType = mime.TypeByExtension(filepath.Ext(args[1]))
		if *contentType == "" {
			*contentType = http.DetectContentType(data)
		}
	}
	*contentType = strings.Split(*contentType, ";")[0]
	if !strings.HasPrefix(*contentType, "image/") && !strings.HasPrefix(*contentType, "video/") && !strings.HasPrefix(*contentType, "audio/") {
		return &exitError{2, "choose an image, video or audio MIME type"}
	}
	var response any
	if err := a.client.request(a.ctx, "POST", "/api/generate/assets", bytes.NewReader(data), map[string]string{"Content-Type": *contentType, "X-Filename": url.PathEscape(filepath.Base(args[1]))}, &response); err != nil {
		return err
	}
	return a.emit(response)
}
func (a *app) voices(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return &exitError{2, "usage: voices list [--scope mine|explore] [--language CODE] [--page N]"}
	}
	f := a.flags("voices list")
	scope := f.String("scope", "mine", "mine or explore")
	language := f.String("language", "", "language code")
	page := f.Int("page", 1, "page")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *page < 1 || (*scope != "mine" && *scope != "explore") {
		return &exitError{2, "invalid voice list options"}
	}
	if err := a.needAuth(); err != nil {
		return err
	}
	q := url.Values{"scope": {*scope}, "page": {strconv.Itoa(*page)}}
	if *language != "" {
		q.Set("language", *language)
	}
	var response any
	if err := a.client.json(a.ctx, "GET", "/api/generate/voices?"+q.Encode(), nil, nil, &response); err != nil {
		return err
	}
	return a.emit(response)
}
