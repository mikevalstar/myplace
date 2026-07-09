package main

import "testing"

// The TTY half of interactive() can't be exercised under `go test` (no PTY),
// but the agent-env half can — and it must win regardless of TTY state.

func TestAgentShell(t *testing.T) {
	for _, v := range agentEnvVars {
		t.Setenv(v, "")
	}
	if agentShell() {
		t.Fatal("agentShell() = true with every agent env var empty")
	}
	t.Setenv("CLAUDECODE", "1")
	if !agentShell() {
		t.Fatal("agentShell() = false with CLAUDECODE=1")
	}
}

func TestInteractiveFalseInAgentShell(t *testing.T) {
	t.Setenv("CURSOR_AGENT", "1")
	if interactive() {
		t.Fatal("interactive() = true with CURSOR_AGENT set")
	}
}
