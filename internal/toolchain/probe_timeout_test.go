package toolchain

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mikevalstar/myplace/internal/run"
)

// TestOutdatedHonorsLookupTimeout is a live, opt-in check that a black-holed
// network degrades to "couldn't check" within the lookup budget instead of
// hanging the dashboard. Skipped by default because it needs the real binaries
// and a real (broken) network; run with:
//
//	HTTPS_PROXY=http://10.255.255.1:9 go test ./internal/toolchain -run Timeout -tags= -v -count=1 -args live
func TestOutdatedHonorsLookupTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("live network check")
	}
	live := false
	for _, a := range testArgs() {
		if a == "live" {
			live = true
		}
	}
	if !live {
		t.Skip("pass -args live to run the live timeout check")
	}
	start := time.Now()
	_, err := New(run.Exec{}).Outdated(context.Background())
	elapsed := time.Since(start)
	t.Logf("elapsed=%s err=%v", elapsed.Round(time.Millisecond), err)
	if elapsed > 4*lookupTimeout {
		t.Errorf("Outdated took %s, want under %s — probes must not hang", elapsed, 4*lookupTimeout)
	}
}

// testArgs is os.Args, isolated so the test above reads cleanly.
func testArgs() []string { return os.Args }
