package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_RELAY_CONFIG_DIR", dir)
	return dir
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	testDir(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultTimeout != "10m" {
		t.Errorf("DefaultTimeout = %q, want 10m", cfg.DefaultTimeout)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := testDir(t)
	in := Default()
	in.Agents = map[string]Agent{
		"codex": {Model: "gpt-5.2", Sandbox: "workspace-write"},
	}
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Fatalf("config file missing: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := out.Agents["codex"]
	if got.Model != "gpt-5.2" || got.Sandbox != "workspace-write" {
		t.Errorf("codex agent = %+v, want model gpt-5.2 sandbox workspace-write", got)
	}
}

func TestTimeoutAgentOverride(t *testing.T) {
	testDir(t)
	cfg := Default()
	cfg.Agents = map[string]Agent{"claude": {Timeout: "2m"}}
	d, err := cfg.Timeout("claude")
	if err != nil {
		t.Fatalf("Timeout: %v", err)
	}
	if d != 2*time.Minute {
		t.Errorf("claude timeout = %v, want 2m", d)
	}
	d, err = cfg.Timeout("codex")
	if err != nil {
		t.Fatalf("Timeout: %v", err)
	}
	if d != 10*time.Minute {
		t.Errorf("codex timeout = %v, want default 10m", d)
	}
}

func TestTimeoutInvalid(t *testing.T) {
	testDir(t)
	cfg := Default()
	cfg.DefaultTimeout = "soon"
	if _, err := cfg.Timeout("claude"); err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestSandboxDefaultsToReadOnlyForCodex(t *testing.T) {
	testDir(t)
	cfg := Default()
	if got := cfg.Sandbox("codex"); got != "read-only" {
		t.Errorf("codex sandbox = %q, want read-only", got)
	}
	if got := cfg.Sandbox("claude"); got != "" {
		t.Errorf("claude sandbox = %q, want empty", got)
	}
}

func TestSessionStorageConfiguration(t *testing.T) {
	dir := testDir(t)
	cfg, err := Load()
	if err != nil || cfg.SessionStorage != "json" {
		t.Fatalf("default storage: %+v, %v", cfg, err)
	}
	cfg.SessionStorage = "sqlite"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil || cfg.SessionStorage != "sqlite" {
		t.Fatalf("SQLite storage: %+v, %v", cfg, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("session_storage = \"sqilte\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid storage error")
	}
}
