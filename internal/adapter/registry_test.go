package adapter

import "testing"

func TestRegistryContainsAllAdapters(t *testing.T) {
	names := Names()
	want := []string{"claude", "codex", "opencode"}
	if len(names) != len(want) {
		t.Fatalf("Names() = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("Names()[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestGetUnknownAgent(t *testing.T) {
	if _, err := Get("gemini"); err == nil {
		t.Error("expected error for unknown agent")
	}
}
