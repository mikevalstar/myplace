package sysinfo

import "testing"

// A trimmed but representative `fastfetch --format json` payload: includes the
// Title/Separator noise fastfetch emits (Separator carries an "error" and no
// result) to prove we skip non-data modules.
const macSample = `[
  {"type":"Title","result":{"hostName":"mbp.local"}},
  {"type":"Separator","error":"Unsupported for JSON format"},
  {"type":"OS","result":{"buildID":"25F80","codename":"Tahoe","id":"macos","name":"macOS","prettyName":"macOS Tahoe 26.5.1 (25F80)","version":"26.5.1"}},
  {"type":"Host","result":{"family":"Mac16,8","name":"MacBook Pro (14-inch, 2024)","vendor":"Apple Inc."}},
  {"type":"Kernel","result":{"architecture":"arm64","name":"Darwin","release":"25.5.0"}},
  {"type":"CPU","result":{"cpu":"Apple M4 Pro","vendor":"Apple","cores":{"physical":12,"logical":12,"online":12}}},
  {"type":"GPU","result":[{"name":"Apple M4 Pro","type":"Integrated","vendor":"Apple"}]},
  {"type":"Memory","result":{"total":25769803776,"used":23112105984}},
  {"type":"Swap","result":[{"name":"Encrypted","total":11811160064,"used":11044782080}]},
  {"type":"Disk","result":[
    {"name":"Macintosh HD","mountpoint":"/","bytes":{"total":494384795648,"used":465891303424},"volumeType":["Regular","Read-only"]},
    {"name":"Data","mountpoint":"/System/Volumes/Data","bytes":{"total":494384795648,"used":465891303424},"volumeType":["Regular"]},
    {"name":"Preboot","mountpoint":"/System/Volumes/Preboot","bytes":{"total":494384795648,"used":465891303424},"volumeType":["Regular"]},
    {"name":"OmniDiskSweeper","mountpoint":"/Volumes/OmniDiskSweeper","bytes":{"total":24117248,"used":18874368},"volumeType":["Read-only"]},
    {"name":"Backup","mountpoint":"/Volumes/Backup","bytes":{"total":2000000000000,"used":500000000000},"volumeType":["Regular"]}
  ]},
  {"type":"Battery","result":[{"capacity":80.0,"cycleCount":92,"status":["AC Connected"]}]},
  {"type":"LocalIp","result":[{"name":"en0","ipv4":"10.0.20.23/22"}]},
  {"type":"Uptime","result":{"uptime":716159060,"bootTime":"2026-06-08T08:14:09.742-0400"}}
]`

// A Linux server: no Battery and no GPU modules at all — must degrade to zero
// values, not an error.
const serverSample = `[
  {"type":"OS","result":{"id":"debian","name":"Debian","prettyName":"Debian GNU/Linux 12 (bookworm)","version":"12","codename":"bookworm"}},
  {"type":"Host","result":{"name":"PowerEdge R640","vendor":"Dell Inc."}},
  {"type":"Kernel","result":{"architecture":"x86_64","name":"Linux","release":"6.1.0-21-amd64"}},
  {"type":"CPU","result":{"cpu":"Intel Xeon Gold 6230","vendor":"Intel","cores":{"physical":20,"logical":40}}},
  {"type":"Memory","result":{"total":137438953472,"used":42949672960}},
  {"type":"Disk","result":[{"name":"root","mountpoint":"/","bytes":{"total":1000204886016,"used":250051221504}}]},
  {"type":"LocalIp","result":[{"name":"eno1","ipv4":"192.168.10.5"}]},
  {"type":"Uptime","result":{"uptime":987654321,"bootTime":"2026-05-01T00:00:00.000+0000"}}
]`

func TestParseMac(t *testing.T) {
	info, err := Parse([]byte(macSample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if info.OS.Name != "macOS" || info.OS.Version != "26.5.1" || info.OS.Build != "25F80" {
		t.Errorf("OS = %+v", info.OS)
	}
	if info.OS.Pretty != "macOS Tahoe 26.5.1 (25F80)" {
		t.Errorf("OS.Pretty = %q", info.OS.Pretty)
	}
	if info.Host.Name != "MacBook Pro (14-inch, 2024)" || info.Host.Vendor != "Apple Inc." {
		t.Errorf("Host = %+v", info.Host)
	}
	if info.Kernel.Arch != "arm64" {
		t.Errorf("Kernel.Arch = %q", info.Kernel.Arch)
	}
	if info.CPU.Name != "Apple M4 Pro" || info.CPU.CoresPhysical != 12 || info.CPU.CoresLogical != 12 {
		t.Errorf("CPU = %+v", info.CPU)
	}
	if len(info.GPUs) != 1 || info.GPUs[0].Name != "Apple M4 Pro" {
		t.Errorf("GPUs = %+v", info.GPUs)
	}
	if info.Memory.TotalBytes != 25769803776 {
		t.Errorf("Memory.TotalBytes = %d", info.Memory.TotalBytes)
	}
	// The disk filter must keep root "/" (even though it's Read-only) and a
	// real external read-write volume, and drop the /System/Volumes/* synthetic
	// mounts and the read-only DMG.
	if len(info.Disks) != 2 {
		t.Fatalf("Disks: want 2 (root + external), got %d: %+v", len(info.Disks), info.Disks)
	}
	if info.Disks[0].Mountpoint != "/" || info.Disks[0].TotalBytes != 494384795648 {
		t.Errorf("Disks[0] = %+v", info.Disks[0])
	}
	if info.Disks[1].Mountpoint != "/Volumes/Backup" {
		t.Errorf("Disks[1] = %+v (expected external /Volumes/Backup)", info.Disks[1])
	}
	if info.Battery == nil || info.Battery.Capacity != 80.0 || info.Battery.CycleCount != 92 {
		t.Errorf("Battery = %+v", info.Battery)
	}
	// CIDR suffix must be stripped.
	if len(info.Network) != 1 || info.Network[0].IPv4 != "10.0.20.23" || info.Network[0].Interface != "en0" {
		t.Errorf("Network = %+v", info.Network)
	}
	if info.Uptime.BootTime == "" || info.Uptime.UptimeMs != 716159060 {
		t.Errorf("Uptime = %+v", info.Uptime)
	}
}

func TestParseServerMissingModules(t *testing.T) {
	info, err := Parse([]byte(serverSample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if info.OS.Name != "Debian" || info.Kernel.Arch != "x86_64" {
		t.Errorf("OS/Kernel = %+v / %+v", info.OS, info.Kernel)
	}
	if info.CPU.CoresLogical != 40 {
		t.Errorf("CPU.CoresLogical = %d", info.CPU.CoresLogical)
	}
	// The whole point: absent modules are zero values, not errors.
	if info.Battery != nil {
		t.Errorf("Battery should be nil on a server, got %+v", info.Battery)
	}
	if len(info.GPUs) != 0 {
		t.Errorf("GPUs should be empty on a server, got %+v", info.GPUs)
	}
	if len(info.Disks) != 1 || info.Disks[0].UsedBytes != 250051221504 {
		t.Errorf("Disks = %+v", info.Disks)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("expected error on non-JSON input")
	}
}
