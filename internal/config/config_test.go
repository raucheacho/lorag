package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	c := &Config{}
	notice, err := c.ResolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".lorag", "data"); c.DataDir != want || notice != "" {
		t.Fatalf("default: got %q (%q), want %q", c.DataDir, notice, want)
	}
	if _, err := os.Stat(c.DataDir); err != nil {
		t.Fatal("data directory not created:", err)
	}

	// A database left in ./data by an earlier version is kept.
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "lorag.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c = &Config{}
	if notice, _ = c.ResolveDataDir(); filepath.Base(c.DataDir) != "data" || notice == "" {
		t.Fatalf("legacy: got %q (%q)", c.DataDir, notice)
	}
	if !filepath.IsAbs(c.DataDir) {
		t.Fatalf("not absolute: %q", c.DataDir)
	}

	// Once the new default holds a database, it wins.
	if err := os.WriteFile(filepath.Join(home, ".lorag", "data", "lorag.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c = &Config{}
	if notice, _ = c.ResolveDataDir(); c.DataDir != filepath.Join(home, ".lorag", "data") || notice != "" {
		t.Fatalf("both: got %q (%q)", c.DataDir, notice)
	}

	// An explicit directory is used as given, made absolute.
	c = &Config{DataDir: "elsewhere"}
	if _, err := c.ResolveDataDir(); err != nil || filepath.Base(c.DataDir) != "elsewhere" || !filepath.IsAbs(c.DataDir) {
		t.Fatalf("explicit: got %q, %v", c.DataDir, err)
	}
}
