package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skills/letsgen/SKILL.md
var skillText string

func (a *app) skills(args []string) error {
	if len(args) == 1 && args[0] == "list" {
		return a.emit(map[string]any{"skills": []string{"letsgen"}, "version": "0.1.0"})
	}
	if len(args) == 1 && args[0] == "show" {
		_, err := fmt.Fprint(a.out, skillText)
		return err
	}
	if len(args) == 0 || args[0] != "install" {
		return &exitError{2, "usage: skills list|show|install --target codex|claude [--path DIR]"}
	}
	f := a.flags("skills install")
	target := f.String("target", "codex", "codex or claude")
	path := f.String("path", "", "agent skill root directory")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || (*target != "codex" && *target != "claude") {
		return &exitError{2, "choose codex or claude"}
	}
	if *path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*path = filepath.Join(home, "."+*target, "skills")
	}
	dest := filepath.Join(*path, "letsgen", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("skill already exists or cannot be created; choose another --path")
	}
	defer file.Close()
	if _, err := file.WriteString(skillText); err != nil {
		return err
	}
	abs, _ := filepath.Abs(dest)
	return a.emit(map[string]string{"installed": abs})
}
