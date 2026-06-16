package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/mikevalstar/myplace/internal/run"
	"github.com/mikevalstar/myplace/internal/sysinfo"
)

// sysinfoEnvelope wraps the curated snapshot with the same schema/machine/
// checked_at header the drift and outdated envelopes carry. The embedded
// *Info flattens its sections (os, host, …) to the top level.
type sysinfoEnvelope struct {
	Schema    int       `json:"schema"`
	Machine   string    `json:"machine"`
	CheckedAt time.Time `json:"checked_at"`
	*sysinfo.Info
}

func newSysinfoCmd(r run.Runner) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "sysinfo",
		Short: "Report this machine's OS and hardware specs (read-only, informational)",
		Long: "Shows OS and version plus base hardware specs (host model, CPU, GPU,\n" +
			"memory, disk) and a few extras (battery, local IP), via fastfetch.\n" +
			"Informational and read-only: it changes nothing and does NOT affect the\n" +
			"drift verdict. Requires fastfetch on PATH (installed via mise).\n" +
			"Exit codes: 0 success, 3 error (fastfetch unavailable or failed).",
		Annotations: map[string]string{
			annHeadless:     "myplace sysinfo --json",
			annExitCodes:    exitCodesSysinfo,
			annOutputSchema: "docs/features/system-information.md",
			annInteractive:  "false",
			annNote:         "informational, read-only snapshot from fastfetch; never mutates and never affects the drift verdict. Requires fastfetch on PATH (in the mise baseline); exits 3 if it isn't.",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := sysinfo.New(r).Fetch(cmd.Context())
			if err != nil {
				logger.Error("sysinfo", "err", err.Error())
				fmt.Fprintln(os.Stderr, "sysinfo: fastfetch could not be run (install it via mise and ensure it's on PATH):", err)
				os.Exit(3)
			}
			logger.Info("sysinfo", "os", info.OS.Name, "host", info.Host.Name)
			if jsonOut {
				machine, _ := os.Hostname()
				emitJSON(sysinfoEnvelope{Schema: 1, Machine: machine, CheckedAt: time.Now().UTC(), Info: info})
			} else {
				fmt.Print(renderSysinfoText(info))
			}
			os.Exit(0)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a single JSON document on stdout")
	return cmd
}

func renderSysinfoText(info *sysinfo.Info) string {
	var b strings.Builder
	line := func(label, val string) {
		if val != "" {
			fmt.Fprintf(&b, "  %-9s %s\n", label+":", val)
		}
	}
	host := info.Host.Name
	if info.Host.Family != "" {
		host += " (" + info.Host.Family + ")"
	}
	osStr := info.OS.Pretty
	if osStr == "" {
		osStr = strings.TrimSpace(info.OS.Name + " " + info.OS.Version)
	}
	line("Host", host)
	line("OS", osStr)
	line("Kernel", strings.TrimSpace(info.Kernel.Name+" "+info.Kernel.Release+" ("+info.Kernel.Arch+")"))

	cpu := info.CPU.Name
	if info.CPU.CoresLogical > 0 {
		cpu = fmt.Sprintf("%s · %d cores", cpu, info.CPU.CoresLogical)
	}
	line("CPU", cpu)
	if len(info.GPUs) > 0 {
		names := make([]string, 0, len(info.GPUs))
		for _, g := range info.GPUs {
			names = append(names, g.Name)
		}
		line("GPU", strings.Join(names, ", "))
	}
	if info.Memory.TotalBytes > 0 {
		line("Memory", memLine(info.Memory.UsedBytes, info.Memory.TotalBytes))
	}
	var swTotal, swUsed uint64
	for _, sw := range info.Swap {
		swTotal += sw.TotalBytes
		swUsed += sw.UsedBytes
	}
	if swTotal > 0 {
		line("Swap", memLine(swUsed, swTotal))
	}
	if len(info.Load) > 0 {
		nums := make([]string, len(info.Load))
		for i, v := range info.Load {
			nums[i] = fmt.Sprintf("%.2f", v)
		}
		line("Load", strings.Join(nums, " "))
	}
	for _, d := range info.Disks {
		line("Disk", fmt.Sprintf("%s  %s / %s (%s)", d.Mountpoint, humanize.IBytes(d.UsedBytes), humanize.IBytes(d.TotalBytes), d.Name))
	}
	if info.Battery != nil {
		bat := fmt.Sprintf("%.0f%%", info.Battery.Capacity)
		if info.Battery.CycleCount > 0 {
			bat += fmt.Sprintf(" · %d cycles", info.Battery.CycleCount)
		}
		if len(info.Battery.Status) > 0 {
			bat += " · " + strings.Join(info.Battery.Status, ", ")
		}
		line("Battery", bat)
	}
	for _, n := range info.Network {
		line("Network", fmt.Sprintf("%s  %s", n.Interface, n.IPv4))
	}
	if info.Uptime.UptimeMs > 0 {
		now := time.Now()
		line("Uptime", strings.TrimSpace(humanize.RelTime(now.Add(-time.Duration(info.Uptime.UptimeMs)*time.Millisecond), now, "", "")))
	}
	machine, _ := os.Hostname()
	return fmt.Sprintf("%s — system info\n%s", machine, b.String())
}

// memLine formats "used / total (free free)" for memory and swap.
func memLine(used, total uint64) string {
	var free uint64
	if total > used {
		free = total - used
	}
	return fmt.Sprintf("%s / %s (%s free)", humanize.IBytes(used), humanize.IBytes(total), humanize.IBytes(free))
}
