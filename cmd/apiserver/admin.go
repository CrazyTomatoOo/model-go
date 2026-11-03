package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	apistore "github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/pkg/store"
)

// runAdmin dispatches the admin subcommands (login design spec §8/§9).
func runAdmin(args []string) error {
	if len(args) != 1 || args[0] != "bootstrap" {
		return usage()
	}
	dsn := os.Getenv("MODELGO_POSTGRES_DSN")
	if dsn == "" {
		return errors.New("MODELGO_POSTGRES_DSN is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	bootstrap := service.NewBootstrapService(apistore.NewUserStore(pool))
	passwords := make([]string, 3)
	for i, label := range []string{"sysadmin", "secadmin", "auditadmin"} {
		passwords[i], err = promptPassword(label + " 初始密码")
		if err != nil {
			return err
		}
	}

	if err := bootstrap.Bootstrap(ctx, passwords[0], passwords[1], passwords[2]); err != nil {
		if errors.Is(err, service.ErrAdminsExist) {
			fmt.Println("管理员账号已存在，无需初始化（幂等拒绝）")
			return nil
		}
		return err
	}
	fmt.Println("已创建三个初始管理员：sysadmin、secadmin、auditadmin（首次登录须修改密码）")
	return nil
}

// promptPassword reads one line from the terminal with echo disabled.
func promptPassword(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	return string(pw), nil
}
