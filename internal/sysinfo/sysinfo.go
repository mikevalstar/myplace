// Package sysinfo wraps fastfetch (ADR-0013) to produce a curated, read-only
// snapshot of the machine: OS/version plus base hardware specs (host model,
// CPU, GPU, memory, disk) and a few fleet-relevant extras (battery, local IP).
// It is TUI-free; the TUI renders an Info and the `sysinfo` command emits it as
// JSON. fastfetch is an optional read dependency — its absence is the caller's
// error to surface, not a panic here.
package sysinfo

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r    run.Runner
	home string
}

func New(r run.Runner) *Client {
	home, _ := os.UserHomeDir()
	return &Client{r: r, home: home}
}

// Info is the curated snapshot. JSON tags define the headless envelope's body
// (the command wraps it with schema/machine/checked_at). Byte counts are raw;
// renderers humanize them.
type Info struct {
	OS      OS       `json:"os"`
	Host    Host     `json:"host"`
	Kernel  Kernel   `json:"kernel"`
	CPU     CPU      `json:"cpu"`
	GPUs    []GPU    `json:"gpus,omitempty"`
	Memory  Memory   `json:"memory"`
	Swap    []Swap   `json:"swap,omitempty"`
	Disks   []Disk   `json:"disks,omitempty"`
	Battery *Battery `json:"battery,omitempty"`
	Network []NetIf  `json:"network,omitempty"`
	Uptime  Uptime   `json:"uptime"`
}

type OS struct {
	Name     string `json:"name"`
	Pretty   string `json:"pretty"`
	Version  string `json:"version"`
	Codename string `json:"codename,omitempty"`
	Build    string `json:"build,omitempty"`
	ID       string `json:"id,omitempty"`
}

type Host struct {
	Name   string `json:"name"`
	Family string `json:"family,omitempty"`
	Vendor string `json:"vendor,omitempty"`
}

type Kernel struct {
	Name    string `json:"name"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
}

type CPU struct {
	Name          string `json:"name"`
	Vendor        string `json:"vendor,omitempty"`
	CoresPhysical int    `json:"cores_physical"`
	CoresLogical  int    `json:"cores_logical"`
}

type GPU struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Vendor string `json:"vendor,omitempty"`
}

type Memory struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

type Swap struct {
	Name       string `json:"name,omitempty"`
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

type Disk struct {
	Name       string `json:"name,omitempty"`
	Mountpoint string `json:"mountpoint"`
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

type Battery struct {
	Capacity   float64  `json:"capacity"`
	CycleCount int      `json:"cycle_count,omitempty"`
	Status     []string `json:"status,omitempty"`
}

type NetIf struct {
	Interface string `json:"interface"`
	IPv4      string `json:"ipv4"`
}

type Uptime struct {
	BootTime string `json:"boot_time,omitempty"`
	UptimeMs uint64 `json:"uptime_ms,omitempty"`
}

// module is one entry of fastfetch's `--format json` array. A module with no
// data carries an "error" string and no usable "result"; we skip those.
type module struct {
	Type   string          `json:"type"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// Parse maps `fastfetch --format json` into a curated Info. It is defensive by
// design: unknown module types are ignored and absent ones (a server with no
// Battery/GPU) leave their fields zero — decoding never fails on a missing
// module, only on output that isn't the fastfetch array at all.
func Parse(out []byte) (*Info, error) {
	var mods []module
	if err := json.Unmarshal(out, &mods); err != nil {
		return nil, err
	}
	info := &Info{}
	for _, m := range mods {
		if len(m.Result) == 0 || string(m.Result) == "null" {
			continue
		}
		switch m.Type {
		case "OS":
			var r struct {
				Name, Version, Codename, BuildID, PrettyName, ID string
			}
			if json.Unmarshal(m.Result, &r) == nil {
				info.OS = OS{Name: r.Name, Pretty: r.PrettyName, Version: r.Version, Codename: r.Codename, Build: r.BuildID, ID: r.ID}
			}
		case "Host":
			var r struct{ Name, Family, Vendor string }
			if json.Unmarshal(m.Result, &r) == nil {
				info.Host = Host{Name: r.Name, Family: r.Family, Vendor: r.Vendor}
			}
		case "Kernel":
			var r struct{ Name, Release, Architecture string }
			if json.Unmarshal(m.Result, &r) == nil {
				info.Kernel = Kernel{Name: r.Name, Release: r.Release, Arch: r.Architecture}
			}
		case "CPU":
			var r struct {
				CPU, Vendor string
				Cores       struct{ Physical, Logical int }
			}
			if json.Unmarshal(m.Result, &r) == nil {
				info.CPU = CPU{Name: r.CPU, Vendor: r.Vendor, CoresPhysical: r.Cores.Physical, CoresLogical: r.Cores.Logical}
			}
		case "GPU":
			var rs []struct{ Name, Type, Vendor string }
			if json.Unmarshal(m.Result, &rs) == nil {
				for _, g := range rs {
					info.GPUs = append(info.GPUs, GPU{Name: g.Name, Type: g.Type, Vendor: g.Vendor})
				}
			}
		case "Memory":
			var r struct{ Total, Used uint64 }
			if json.Unmarshal(m.Result, &r) == nil {
				info.Memory = Memory{TotalBytes: r.Total, UsedBytes: r.Used}
			}
		case "Swap":
			var rs []struct {
				Name        string
				Total, Used uint64
			}
			if json.Unmarshal(m.Result, &rs) == nil {
				for _, s := range rs {
					info.Swap = append(info.Swap, Swap{Name: s.Name, TotalBytes: s.Total, UsedBytes: s.Used})
				}
			}
		case "Disk":
			var rs []struct {
				Name, Mountpoint string
				Bytes            struct{ Total, Used uint64 }
				VolumeType       []string
			}
			if json.Unmarshal(m.Result, &rs) == nil {
				for _, d := range rs {
					if !interestingDisk(d.Mountpoint, d.VolumeType) {
						continue
					}
					info.Disks = append(info.Disks, Disk{Name: d.Name, Mountpoint: d.Mountpoint, TotalBytes: d.Bytes.Total, UsedBytes: d.Bytes.Used})
				}
			}
		case "Battery":
			var rs []struct {
				Capacity   float64
				CycleCount int
				Status     []string
			}
			if json.Unmarshal(m.Result, &rs) == nil && len(rs) > 0 {
				b := rs[0]
				info.Battery = &Battery{Capacity: b.Capacity, CycleCount: b.CycleCount, Status: b.Status}
			}
		case "LocalIp":
			var rs []struct {
				Name string
				IPv4 string
			}
			if json.Unmarshal(m.Result, &rs) == nil {
				for _, n := range rs {
					if n.IPv4 == "" {
						continue
					}
					info.Network = append(info.Network, NetIf{Interface: n.Name, IPv4: stripCIDR(n.IPv4)})
				}
			}
		case "Uptime":
			var r struct {
				Uptime   uint64
				BootTime string
			}
			if json.Unmarshal(m.Result, &r) == nil {
				info.Uptime = Uptime{BootTime: r.BootTime, UptimeMs: r.Uptime}
			}
		}
	}
	return info, nil
}

// Fetch runs `fastfetch --format json` through the runner choke point and
// parses it. An error means fastfetch isn't on PATH or failed; callers fail
// fast (the command) or degrade to a notice (the TUI band).
func (c *Client) Fetch(ctx context.Context) (*Info, error) {
	out, err := c.r.Run(ctx, c.home, "fastfetch", "--format", "json")
	if err != nil {
		return nil, err
	}
	return Parse(out)
}

// interestingDisk keeps the disks worth showing in a specs view and drops the
// noise. On macOS that means hiding the synthetic APFS system volumes
// (/System/Volumes/Preboot, VM, Data, …) that all duplicate the root
// container, and read-only mounts (app DMGs under /Volumes); the root "/" is
// always kept. On Linux this keeps real mounts (/, /home, /boot, external
// media) and drops read-only loop/snap mounts. Cross-platform by mountpoint +
// volume-type heuristics, never OS-specific branching.
func interestingDisk(mountpoint string, volumeType []string) bool {
	if mountpoint == "/" {
		return true
	}
	if strings.HasPrefix(mountpoint, "/System/Volumes/") {
		return false
	}
	for _, t := range volumeType {
		if strings.EqualFold(t, "Read-only") {
			return false
		}
	}
	return true
}

// stripCIDR drops a trailing "/24"-style prefix length from an address.
func stripCIDR(addr string) string {
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		return addr[:i]
	}
	return addr
}
