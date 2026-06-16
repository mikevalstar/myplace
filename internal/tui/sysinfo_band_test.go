package tui

import (
	"strings"
	"testing"

	"github.com/mikevalstar/myplace/internal/sysinfo"
)

func sampleSysinfo() *sysinfo.Info {
	return &sysinfo.Info{
		OS:      sysinfo.OS{Name: "macOS", Pretty: "macOS Tahoe 26.5.1 (25F80)", Version: "26.5.1"},
		Host:    sysinfo.Host{Name: "MacBook Pro (14-inch, 2024)", Vendor: "Apple Inc."},
		Kernel:  sysinfo.Kernel{Name: "Darwin", Release: "25.5.0", Arch: "arm64"},
		CPU:     sysinfo.CPU{Name: "Apple M4 Pro", CoresLogical: 12},
		GPUs:    []sysinfo.GPU{{Name: "Apple M4 Pro"}},
		Memory:  sysinfo.Memory{TotalBytes: 25769803776, UsedBytes: 18000000000},
		Disks:   []sysinfo.Disk{{Name: "Macintosh HD", Mountpoint: "/", TotalBytes: 494384795648, UsedBytes: 465891303424}},
		Battery: &sysinfo.Battery{Capacity: 80, CycleCount: 92, Status: []string{"AC Connected"}},
		Network: []sysinfo.NetIf{{Interface: "en0", IPv4: "10.0.20.23"}},
		Uptime:  sysinfo.Uptime{UptimeMs: 716159060},
	}
}

func TestSysinfoBandLinesContent(t *testing.T) {
	lines := sysinfoBandLinesFor(sampleSysinfo())
	if len(lines) != sysinfoBandLines {
		t.Fatalf("want %d lines, got %d", sysinfoBandLines, len(lines))
	}
	wantPerLine := [][]string{
		{"MacBook Pro", "macOS Tahoe 26.5.1", "arm64"},
		{"Apple M4 Pro", "12 cores", "GPU", "RAM", "Disk / "},
		{"Battery 80%", "92 cyc", "AC Connected", "10.0.20.23", "up "},
	}
	for i, wants := range wantPerLine {
		for _, w := range wants {
			if !strings.Contains(lines[i], w) {
				t.Errorf("band line %d %q missing %q", i, lines[i], w)
			}
		}
	}
}

func TestSysinfoBandFixedHeight(t *testing.T) {
	m := New(nil, nil, nil, nil, "0.1.0")
	// Loading (system nil, no error), error, and populated must all render
	// exactly sysinfoBandLines rows so the layout never shifts.
	for _, tc := range []struct {
		name string
		set  func(*Model)
	}{
		{"loading", func(m *Model) {}},
		{"error", func(m *Model) { m.systemErr = true }},
		{"populated", func(m *Model) { m.system = sampleSysinfo() }},
	} {
		mm := m
		tc.set(&mm)
		got := strings.Count(mm.sysinfoBand(80), "\n") + 1
		if got != sysinfoBandLines {
			t.Errorf("%s: band height = %d rows, want %d", tc.name, got, sysinfoBandLines)
		}
	}
}
