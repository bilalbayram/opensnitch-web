package dnspolicy

// DoHProviderIPs contains IP addresses of well-known DNS-over-HTTPS providers.
// Blocking these on port 443 prevents applications from bypassing local DNS policy.
var DoHProviderIPs = []string{
	// Cloudflare
	"1.1.1.1", "1.0.0.1",
	"2606:4700:4700::1111", "2606:4700:4700::1001",
	// Google
	"8.8.8.8", "8.8.4.4",
	"2001:4860:4860::8888", "2001:4860:4860::8844",
	// Quad9
	"9.9.9.9", "149.112.112.112",
	"2620:fe::fe", "2620:fe::9",
	// OpenDNS
	"208.67.222.222", "208.67.220.220",
	// NextDNS
	"45.90.28.0", "45.90.30.0",
	// AdGuard
	"94.140.14.14", "94.140.15.15",
	// CleanBrowsing
	"185.228.168.9", "185.228.169.9",
	// Mullvad
	"194.242.2.2",
}

// DoHHostnames contains hostnames of well-known DNS-over-HTTPS endpoints.
// These are used to populate the built-in DoH blocklist so the existing
// domain-blocking pipeline can intercept DoH connections by hostname.
var DoHHostnames = []string{
	"dns.google",
	"dns.google.com",
	"cloudflare-dns.com",
	"one.one.one.one",
	"mozilla.cloudflare-dns.com",
	"dns.quad9.net",
	"doh.opendns.com",
	"dns.nextdns.io",
	"dns.adguard-dns.com",
	"doh.cleanbrowsing.org",
	"dns.mullvad.net",
	"doh.mullvad.net",
	"dns.controld.com",
	"freedns.controld.com",
	"dns.sb",
	"doh.dns.sb",
}
