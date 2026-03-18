package router

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// DiscoveredRouter represents a device found during network scan.
type DiscoveredRouter struct {
	IP       string `json:"ip"`
	SSHPort  int    `json:"ssh_port"`
	Banner   string `json:"banner"`
	IsOpenWrt bool  `json:"is_openwrt"`
	Hostname string `json:"hostname,omitempty"`
}

// ScanSubnet scans the given CIDR for devices with SSH (port 22) open
// and identifies OpenWrt routers by their Dropbear SSH banner.
func ScanSubnet(cidr string) ([]DiscoveredRouter, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	var ips []string
	for addr := ip.Mask(ipNet.Mask); ipNet.Contains(addr); incIP(addr) {
		ips = append(ips, addr.String())
	}

	// Skip network and broadcast for /24 and larger
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	// Cap to /22 (1022 hosts) to avoid abuse
	if len(ips) > 1022 {
		ips = ips[:1022]
	}

	const concurrency = 50
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var results []DiscoveredRouter
	var wg sync.WaitGroup

	for _, target := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			if d := probeSSH(ip, 22); d != nil {
				mu.Lock()
				results = append(results, *d)
				mu.Unlock()
			}
		}(target)
	}

	wg.Wait()
	return results, nil
}

// DetectLocalSubnet returns the CIDR of the first non-loopback private IPv4 interface.
func DetectLocalSubnet() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			if isPrivateIP(ip4) {
				// Normalize to network address
				network := ip4.Mask(ipNet.Mask)
				ones, _ := ipNet.Mask.Size()
				return fmt.Sprintf("%s/%d", network.String(), ones), nil
			}
		}
	}

	return "", fmt.Errorf("no private IPv4 interface found")
}

func probeSSH(ip string, port int) *DiscoveredRouter {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 1500*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()

	// Read SSH banner with a tight timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil
	}
	banner := strings.TrimSpace(scanner.Text())
	if banner == "" {
		return nil
	}

	isOpenWrt := strings.Contains(strings.ToLower(banner), "dropbear")

	return &DiscoveredRouter{
		IP:        ip,
		SSHPort:   port,
		Banner:    banner,
		IsOpenWrt: isOpenWrt,
	}
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	n := binary.BigEndian.Uint32(ip4)
	// 10.0.0.0/8
	if n>>24 == 10 {
		return true
	}
	// 172.16.0.0/12
	if n>>20 == 0xAC1 {
		return true
	}
	// 192.168.0.0/16
	if n>>16 == 0xC0A8 {
		return true
	}
	return false
}
