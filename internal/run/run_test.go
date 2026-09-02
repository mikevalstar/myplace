package run

import (
	"context"
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	// A non-zero exit surfaces its code and stderr through run.Error, and the
	// message keeps the "<cmd> <args>: <stderr-or-cause>" shape callers log.
	_, err := Exec{}.Run(context.Background(), "", "sh", "-c", "echo oops >&2; exit 2")
	if err == nil {
		t.Fatal("expected an error")
	}
	code, stderr, ok := ExitCode(err)
	if !ok || code != 2 || stderr != "oops" {
		t.Fatalf("ExitCode = %d, %q, %v; want 2, \"oops\", true", code, stderr, ok)
	}
	if err.Error() != "sh -c echo oops >&2; exit 2: oops" {
		t.Errorf("message = %q", err.Error())
	}

	// A silent status exit carries an empty stderr and the cause as message.
	_, err = Exec{}.Run(context.Background(), "", "sh", "-c", "exit 3")
	if code, stderr, ok := ExitCode(err); !ok || code != 3 || stderr != "" {
		t.Fatalf("silent exit: ExitCode = %d, %q, %v", code, stderr, ok)
	}

	// Not on PATH: no exit code to report.
	_, err = Exec{}.Run(context.Background(), "", "definitely-not-a-command-xyz")
	if _, _, ok := ExitCode(err); ok {
		t.Error("a process that never ran must not report an exit code")
	}
	if _, _, ok := ExitCode(errors.New("plain")); ok {
		t.Error("a plain error must not report an exit code")
	}
	if _, _, ok := ExitCode(nil); ok {
		t.Error("nil must not report an exit code")
	}
}
