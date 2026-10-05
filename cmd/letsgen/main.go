package main

import (
	"context"
	"github.com/LetsGenLab/letsgen-cli/internal/cli"
	"os"
	"os/signal"
	"syscall"
)

var version = "0.1.0-dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(cli.Main(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version))
}
