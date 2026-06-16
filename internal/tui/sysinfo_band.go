package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/mikevalstar/myplace/internal/sysinfo"
)

// sysinfoBandLines is the fixed height (in rows) of the system-info header
// band. Fixed so the dashboard layout never shifts as the snapshot loads or
// when fastfetch is unavailable.
const sysinfoBandLines = 3

// sysinfoBand renders the passive system-info header band: three compact,
// width-truncated lines drawn from the fastfetch snapshot. While loading it
// shows a placeholder; if fastfetch is unavailable it shows a single notice.
// It always returns exactly sysinfoBandLines rows.
func (m Model) sysinfoBand(width int) string {
	th := m.theme
	var lines []string
	switch {
	case m.systemErr:
		lines = []string{"system: fastfetch unavailable"}
	case m.system == nil:
		lines = []string{"system: loading…"}
	default:
		lines = sysinfoBandLinesFor(m.system)
	}
	// Always emit exactly sysinfoBandLines rows so the layout height is stable.
	out := make([]string, sysinfoBandLines)
	for i := 0; i < sysinfoBandLines; i++ {
		if i < len(lines) {
			out[i] = th.Subtle.Render(truncate(lines[i], width))
		}
	}
	return strings.Join(out, "\n")
}

// sysinfoBandLinesFor builds the three plain-text content lines.
func sysinfoBandLinesFor(s *sysinfo.Info) []string {
	join := func(parts ...string) string {
		kept := parts[:0]
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				kept = append(kept, p)
			}
		}
		return strings.Join(kept, " · ")
	}

	// L1 — identity: host model · OS · arch
	host := s.Host.Name
	osStr := s.OS.Pretty
	if osStr == "" {
		osStr = strings.TrimSpace(s.OS.Name + " " + s.OS.Version)
	}
	l1 := join(host, osStr, s.Kernel.Arch)

	// L2 — compute: CPU · cores · GPU · RAM · root disk
	cores := ""
	if s.CPU.CoresLogical > 0 {
		cores = fmt.Sprintf("%d cores", s.CPU.CoresLogical)
	}
	gpu := ""
	if len(s.GPUs) > 0 {
		gpu = "GPU " + s.GPUs[0].Name
	}
	ram := ""
	if s.Memory.TotalBytes > 0 {
		ram = "RAM " + humanize.IBytes(s.Memory.TotalBytes)
	}
	disk := ""
	if d, ok := rootDisk(s); ok {
		disk = fmt.Sprintf("Disk %s %s/%s", d.Mountpoint, humanize.IBytes(d.UsedBytes), humanize.IBytes(d.TotalBytes))
	}
	l2 := join(s.CPU.Name, cores, gpu, ram, disk)

	// L3 — power/net: battery · IPv4 · uptime
	bat := ""
	if s.Battery != nil {
		bat = fmt.Sprintf("Battery %.0f%%", s.Battery.Capacity)
		extra := []string{}
		if s.Battery.CycleCount > 0 {
			extra = append(extra, fmt.Sprintf("%d cyc", s.Battery.CycleCount))
		}
		if len(s.Battery.Status) > 0 {
			extra = append(extra, strings.Join(s.Battery.Status, ", "))
		}
		if len(extra) > 0 {
			bat += " (" + strings.Join(extra, " · ") + ")"
		}
	}
	ip := ""
	if len(s.Network) > 0 {
		ip = s.Network[0].IPv4
	}
	up := ""
	if s.Uptime.UptimeMs > 0 {
		up = "up " + formatUptime(time.Duration(s.Uptime.UptimeMs)*time.Millisecond)
	}
	l3 := join(bat, ip, up)

	return []string{l1, l2, l3}
}

// rootDisk returns the "/" mount if present, else the first disk.
func rootDisk(s *sysinfo.Info) (sysinfo.Disk, bool) {
	if len(s.Disks) == 0 {
		return sysinfo.Disk{}, false
	}
	for _, d := range s.Disks {
		if d.Mountpoint == "/" {
			return d, true
		}
	}
	return s.Disks[0], true
}

// formatUptime renders a duration compactly, e.g. "7d 22h" or "3h 12m".
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
