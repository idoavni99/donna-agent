package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFileLoadsValuesWithoutOverridingShell(t *testing.T) {
	t.Setenv("DONNA_TEST_ENV_VALUE", "from-shell")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DONNA_TEST_ENV_VALUE=from-file\nDONNA_TEST_ENV_NEW=loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DONNA_TEST_ENV_VALUE"); got != "from-shell" {
		t.Fatalf(".env overrode shell variable: %q", got)
	}
	if got := os.Getenv("DONNA_TEST_ENV_NEW"); got != "loaded" {
		t.Fatalf(".env value not loaded: %q", got)
	}
}

func TestLoadEnvFileTreatsMissingFileAsOptional(t *testing.T) {
	if err := loadEnvFile(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("missing .env should be optional: %v", err)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{input: "", want: slog.LevelInfo},
		{input: "DEBUG", want: slog.LevelDebug},
		{input: "warn", want: slog.LevelWarn},
		{input: "error", want: slog.LevelError},
	}
	for _, test := range tests {
		got, err := parseLogLevel(test.input)
		if err != nil || got != test.want {
			t.Errorf("parseLogLevel(%q) = %v, %v; want %v", test.input, got, err, test.want)
		}
	}
	if _, err := parseLogLevel("verbose"); err == nil {
		t.Fatal("expected unsupported log level to be rejected")
	}
}
