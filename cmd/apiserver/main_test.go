package main

import (
	"strings"
	"testing"
)

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"unknown"},
		{"admin"},
		{"admin", "unknown"},
		{"admin", "bootstrap", "extra"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := run(args)
			if err == nil {
				t.Fatalf("run(%v) must return an error", args)
			}
			if !strings.Contains(err.Error(), "expected subcommand") {
				t.Fatalf("run(%v) error = %q, want usage error", args, err)
			}
		})
	}
}
