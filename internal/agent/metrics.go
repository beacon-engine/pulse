package agent

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/unix"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports the hardware-metrics half of agent/src/docker.rs (sysinfo → gopsutil,
// with the same HOST_ROOT / statfs disk logic).

func hardwareMetrics(hostRoot string) protocol.HardwareMetrics {
	var memUsed, memTotal uint64
	var memPct float32
	if vm, err := mem.VirtualMemory(); err == nil {
		memTotal = vm.Total
		memUsed = vm.Used
		if memTotal > 0 {
			memPct = float32(float64(memUsed) / float64(memTotal) * 100.0)
		}
	}
	return protocol.HardwareMetrics{
		MemUsed:  memUsed,
		MemTotal: memTotal,
		MemPct:   memPct,
		Disks:    diskMetrics(hostRoot),
	}
}

// diskMetrics chooses the collector honestly: HOST_ROOT bind-mount → real host
// disks; containerized without it → nothing (the overlay fs is misleading);
// bare-metal → the local machine.
func diskMetrics(hostRoot string) []protocol.DiskInfo {
	if hostRoot != "" && hostRoot != "/" {
		return hostDiskMetrics(hostRoot)
	}
	if inContainer() {
		fmt.Fprintln(os.Stderr, "host disk metrics unavailable: running in a container without HOST_ROOT. "+
			"Bind-mount the host (-v /:/host:ro) and set HOST_ROOT=/host to enable them.")
		return nil
	}
	return localDiskMetrics()
}

// Pseudo/virtual filesystems we never report.
var skipFS = map[string]bool{
	"proc": true, "sysfs": true, "tmpfs": true, "devtmpfs": true, "devpts": true,
	"cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true, "mqueue": true,
	"fusectl": true, "debugfs": true, "tracefs": true, "bpf": true, "configfs": true,
	"pstore": true, "securityfs": true, "ramfs": true, "autofs": true, "binfmt_misc": true,
	"nsfs": true, "hugetlbfs": true, "fuse.lxcfs": true,
}

type diskEntry struct {
	prio int
	info protocol.DiskInfo
}

// hostDiskMetrics reads the host mount table through HOST_ROOT and statfs's each
// real filesystem via the bind-mount.
func hostDiskMetrics(root string) []protocol.DiskInfo {
	data, err := os.ReadFile(root + "/proc/1/mounts")
	if err != nil {
		data, err = os.ReadFile(root + "/proc/mounts")
	}
	if err != nil {
		return localDiskMetrics()
	}

	best := map[[2]uint64]diskEntry{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		device := f[0]
		mount := strings.ReplaceAll(f[1], `\040`, " ") // octal-escaped spaces
		fstype := f[2]

		if skipFS[fstype] {
			continue
		}
		isReal := strings.HasPrefix(device, "/dev/") ||
			fstype == "zfs" || fstype == "nfs" || fstype == "nfs4" || fstype == "cifs" || fstype == "ceph"
		if !isReal {
			continue
		}

		path := root
		if mount != "/" {
			path = root + mount
		}
		total, avail, ok := statfsBytes(path)
		if !ok || total < 1_000_000_000 {
			continue
		}
		prio := 10
		if mount == "/" {
			prio = 100
		}
		putDisk(best, prio, device, mount, total, avail)
	}
	return sortedDisks(best)
}

// localDiskMetrics collects disks on the machine the agent runs on (bare-metal
// dev), deduplicating volumes that share physical storage (e.g. macOS APFS).
func localDiskMetrics() []protocol.DiskInfo {
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil
	}
	best := map[[2]uint64]diskEntry{}
	for _, p := range parts {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage.Total < 1_000_000_000 {
			continue
		}
		mount := p.Mountpoint
		if strings.Contains(mount, "AppTranslocation") ||
			strings.Contains(mount, "CoreSimulator") ||
			strings.HasPrefix(mount, "/private/var/") ||
			strings.HasPrefix(mount, "/System/Volumes/") {
			continue
		}
		prio := 10
		switch {
		case mount == "/":
			prio = 100
		case strings.HasPrefix(mount, "/Volumes/"):
			prio = 50
		}
		putDisk(best, prio, p.Device, mount, usage.Total, usage.Free)
	}
	return sortedDisks(best)
}

// putDisk deduplicates by (total, available) — bind mounts / shared volumes
// report identical stats — keeping the highest-priority mount.
func putDisk(best map[[2]uint64]diskEntry, prio int, name, mount string, total, avail uint64) {
	var used uint64
	if total > avail {
		used = total - avail
	}
	var pct float32
	if total > 0 {
		pct = float32(float64(used) / float64(total) * 100.0)
	}
	info := protocol.DiskInfo{Name: name, Mount: mount, Total: total, Used: used, Pct: pct}
	key := [2]uint64{total, avail}
	if e, ok := best[key]; !ok || prio > e.prio {
		best[key] = diskEntry{prio, info}
	}
}

func sortedDisks(best map[[2]uint64]diskEntry) []protocol.DiskInfo {
	result := make([]protocol.DiskInfo, 0, len(best))
	for _, e := range best {
		result = append(result, e.info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Total > result[j].Total })
	return result
}

// statfsBytes returns (total, available) bytes for path, or ok=false on error.
func statfsBytes(path string) (total, avail uint64, ok bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	bsize := uint64(st.Bsize)
	return st.Blocks * bsize, st.Bavail * bsize, true
}
