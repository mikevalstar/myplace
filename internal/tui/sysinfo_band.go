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

// plainSep separates band fields in unstyled output (smallView / tests). The
// colored band uses a themed separator of the same width.
const plainSep = "   ·   "

// bandSeg is one field in the band: a value, optionally prefixed by a colored
// label ("CPU", "RAM", …). head marks the identity headline (the host model).
type bandSeg struct {
	label string
	value string
	head  bool
}

// sysinfoBand renders the passive system-info header band: three compact,
// width-truncated lines drawn from the fastfetch snapshot, with colored labels
// and a highlighted host headline. While loading it shows a placeholder; if
// fastfetch is unavailable it shows a single notice. It always returns exactly
// sysinfoBandLines rows so the layout height is stable.
func (m Model) sysinfoBand(width int) string {
	th := m.theme
	out := make([]string, sysinfoBandLines)
	switch {
	case m.systemErr:
		out[0] = th.Subtle.Render(truncate("system: fastfetch unavailable", width))
	case m.system == nil:
		out[0] = th.Subtle.Render(truncate("system: loading…", width))
	default:
		sep := th.Rule.Render(plainSep)
		segs := sysinfoBandSegments(m.system)
		for i := 0; i < sysinfoBandLines && i < len(segs); i++ {
			parts := make([]string, 0, len(segs[i]))
			for _, sg := range segs[i] {
				switch {
				case sg.label != "":
					parts = append(parts, th.PaneTitle.Render(sg.label)+" "+th.Help.Render(sg.value))
				case sg.head:
					parts = append(parts, th.AccentNeutral.Render(sg.value))
				default:
					parts = append(parts, th.Help.Render(sg.value))
				}
			}
			out[i] = truncate(strings.Join(parts, sep), width)
		}
	}
	return strings.Join(out, "\n")
}

// sysinfoBandSegments builds the three lines as labeled segments — the single
// source of truth for both the colored band and the plain text variant.
func sysinfoBandSegments(s *sysinfo.Info) [][]bandSeg {
	osStr := s.OS.Pretty
	if osStr == "" {
		osStr = strings.TrimSpace(s.OS.Name + " " + s.OS.Version)
	}

	// L1 — identity: OS (headline) · host model · arch. OS leads so it stays
	// visible even when the host model string is long enough to truncate.
	var l1 []bandSeg
	if osStr != "" {
		l1 = append(l1, bandSeg{value: osStr, head: true})
	}
	if s.Host.Name != "" {
		l1 = append(l1, bandSeg{value: s.Host.Name})
	}
	if s.Kernel.Arch != "" {
		l1 = append(l1, bandSeg{value: s.Kernel.Arch})
	}

	// L2 — compute: CPU (cores) · GPU · RAM (used/total/free) · root disk
	var l2 []bandSeg
	cpu := s.CPU.Name
	if s.CPU.CoresLogical > 0 {
		cpu = fmt.Sprintf("%s (%d cores)", cpu, s.CPU.CoresLogical)
	}
	if strings.TrimSpace(cpu) != "" {
		l2 = append(l2, bandSeg{label: "CPU", value: cpu})
	}
	if len(s.GPUs) > 0 && s.GPUs[0].Name != "" {
		l2 = append(l2, bandSeg{label: "GPU", value: s.GPUs[0].Name})
	}
	if s.Memory.TotalBytes > 0 {
		l2 = append(l2, bandSeg{label: "RAM", value: usedTotalFree(s.Memory.UsedBytes, s.Memory.TotalBytes)})
	}
	if d, ok := rootDisk(s); ok {
		l2 = append(l2, bandSeg{label: "Disk", value: fmt.Sprintf("%s %s/%s", d.Mountpoint, humanize.IBytes(d.UsedBytes), humanize.IBytes(d.TotalBytes))})
	}

	// L3 — runtime: load averages · swap (used/total/free) · battery · IPv4 · uptime
	var l3 []bandSeg
	if len(s.Load) > 0 {
		nums := make([]string, len(s.Load))
		for i, v := range s.Load {
			nums[i] = fmt.Sprintf("%.2f", v)
		}
		l3 = append(l3, bandSeg{label: "Load", value: strings.Join(nums, " ")})
	}
	var swTotal, swUsed uint64
	for _, sw := range s.Swap {
		swTotal += sw.TotalBytes
		swUsed += sw.UsedBytes
	}
	if swTotal > 0 {
		l3 = append(l3, bandSeg{label: "Swap", value: usedTotalFree(swUsed, swTotal)})
	}
	if s.Battery != nil {
		bat := fmt.Sprintf("%.0f%%", s.Battery.Capacity)
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
		l3 = append(l3, bandSeg{label: "Battery", value: bat})
	}
	if len(s.Network) > 0 && s.Network[0].IPv4 != "" {
		l3 = append(l3, bandSeg{label: "IP", value: s.Network[0].IPv4})
	}
	if s.Uptime.UptimeMs > 0 {
		l3 = append(l3, bandSeg{label: "up", value: formatUptime(time.Duration(s.Uptime.UptimeMs) * time.Millisecond)})
	}

	return [][]bandSeg{l1, l2, l3}
}

// sysinfoBandLinesFor renders the band as plain (unstyled) text lines, used by
// the small-terminal fallback and tests.
func sysinfoBandLinesFor(s *sysinfo.Info) []string {
	segs := sysinfoBandSegments(s)
	lines := make([]string, len(segs))
	for i, line := range segs {
		parts := make([]string, 0, len(line))
		for _, sg := range line {
			if sg.label != "" {
				parts = append(parts, sg.label+" "+sg.value)
			} else {
				parts = append(parts, sg.value)
			}
		}
		lines[i] = strings.Join(parts, plainSep)
	}
	return lines
}

// usedTotalFree formats a "used / total (free free)" string from byte counts,
// clamping free at 0 in case used somehow exceeds total.
func usedTotalFree(used, total uint64) string {
	var free uint64
	if total > used {
		free = total - used
	}
	return fmt.Sprintf("%s / %s (%s free)", humanize.IBytes(used), humanize.IBytes(total), humanize.IBytes(free))
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
