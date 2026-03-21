package router

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	minKernelVersionMajor = 5
	minKernelVersionMinor = 4
	minRAMMB              = 128
	minOverlayFreeMB      = 20
)

type RouterCapabilities struct {
	OpenWrt          bool   `json:"openwrt"`
	Arch             string `json:"arch"`
	KernelVersion    string `json:"kernel_version"`
	KernelSupported  bool   `json:"kernel_supported"`
	RAMMB            int    `json:"ram_mb"`
	RAMSupported     bool   `json:"ram_supported"`
	OverlayFreeMB    int    `json:"overlay_free_mb"`
	OverlaySupported bool   `json:"overlay_supported"`
	HasNFT           bool   `json:"has_nft"`
	HasConntrack     bool   `json:"has_conntrack"`
	HasOpkg          bool   `json:"has_opkg"`
	Eligible         bool   `json:"eligible"`
	IneligibleReason string `json:"ineligible_reason"`
}

type CapabilityCheckResult struct {
	Capabilities *RouterCapabilities `json:"capabilities"`
	Steps        []ProvisionStep     `json:"steps"`
}

func CheckCapabilities(client remoteClient) (*RouterCapabilities, error) {
	caps := &RouterCapabilities{}

	openwrtOut, err := client.Run("cat /etc/openwrt_release 2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("check openwrt release: %w", err)
	}
	caps.OpenWrt = strings.Contains(openwrtOut, "DISTRIB")
	if !caps.OpenWrt {
		caps.IneligibleReason = "router-daemon v1 only supports OpenWrt"
		return finalizeCapabilities(caps), nil
	}

	if caps.Arch, err = runTrimmed(client, "uname -m"); err != nil {
		return nil, fmt.Errorf("check architecture: %w", err)
	}
	if caps.KernelVersion, err = runTrimmed(client, "uname -r"); err != nil {
		return nil, fmt.Errorf("check kernel version: %w", err)
	}
	caps.KernelSupported = kernelAtLeast(caps.KernelVersion, minKernelVersionMajor, minKernelVersionMinor)

	memKB, err := runTrimmed(client, "awk '/MemTotal/ {print $2}' /proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("check memory: %w", err)
	}
	if parsed, err := strconv.Atoi(memKB); err == nil {
		caps.RAMMB = parsed / 1024
	}
	caps.RAMSupported = caps.RAMMB >= minRAMMB

	overlayKB, err := runTrimmed(client, "df -k /overlay 2>/dev/null | awk 'NR==2 {print $4}'")
	if err != nil {
		return nil, fmt.Errorf("check overlay free space: %w", err)
	}
	if parsed, err := strconv.Atoi(overlayKB); err == nil {
		caps.OverlayFreeMB = parsed / 1024
	}
	caps.OverlaySupported = caps.OverlayFreeMB >= minOverlayFreeMB

	caps.HasNFT = commandExists(client, "nft")
	caps.HasConntrack = commandExists(client, "conntrack")
	caps.HasOpkg = commandExists(client, "opkg")

	return finalizeCapabilities(caps), nil
}

func finalizeCapabilities(caps *RouterCapabilities) *RouterCapabilities {
	switch {
	case !caps.OpenWrt:
		caps.IneligibleReason = "router-daemon v1 only supports OpenWrt"
	case caps.Arch != "aarch64":
		caps.IneligibleReason = "router-daemon v1 only supports aarch64 OpenWrt targets"
	case !caps.KernelSupported:
		caps.IneligibleReason = "router-daemon v1 requires kernel 5.4 or newer"
	case !caps.RAMSupported:
		caps.IneligibleReason = "router-daemon v1 requires at least 128MB RAM"
	case !caps.OverlaySupported:
		caps.IneligibleReason = "router-daemon v1 requires at least 20MB free overlay space"
	case !caps.HasNFT:
		caps.IneligibleReason = "router-daemon v1 requires nft"
	case !caps.HasConntrack:
		caps.IneligibleReason = "router-daemon v1 requires conntrack"
	case !caps.HasOpkg:
		caps.IneligibleReason = "router-daemon v1 requires opkg"
	default:
		caps.Eligible = true
		caps.IneligibleReason = ""
	}

	return caps
}

func kernelAtLeast(value string, wantMajor, wantMinor int) bool {
	fields := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == '.' || r == '-'
	})
	if len(fields) < 2 {
		return false
	}

	major, err := strconv.Atoi(fields[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(fields[1])
	if err != nil {
		return false
	}

	if major != wantMajor {
		return major > wantMajor
	}
	return minor >= wantMinor
}

func commandExists(client remoteClient, name string) bool {
	out, err := client.Run("command -v " + name + " >/dev/null 2>&1 && echo YES || echo NO")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "YES"
}

func runTrimmed(client remoteClient, cmd string) (string, error) {
	out, err := client.Run(cmd)
	return strings.TrimSpace(out), err
}
