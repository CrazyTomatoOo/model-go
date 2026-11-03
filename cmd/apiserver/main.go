// Command apiserver is the hub REST surface entrypoint (ADR-0001).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver"
)

func main() {
	if err := run(); err != nil {
		slog.Error("apiserver exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		return usage()
	}
	cfg, err := apiserver.ConfigFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return apiserver.Run(ctx, cfg, logger)
}

func usage() error {
	fmt.Fprint(os.Stderr, `model-go apiserver — hub REST surface (ADR-0001)

Usage:
  apiserver serve

Environment (MODELGO_*):
  MODELGO_HTTP_ADDR     listen address (default ":8080")
  MODELGO_POSTGRES_DSN  Postgres DSN, required (e.g. postgres://user:pass@host:5432/modelgo?sslmode=disable)
  MODELGO_REDIS_ADDR    Redis address, required (e.g. 127.0.0.1:6379)
`)
	return errors.New("expected subcommand: serve")
}
