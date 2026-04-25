package util

import (
	"testing"
)

// net.LookupHost with a numeric IP returns it directly without a DNS query,
// so these tests are safe to run without network access.
func TestValidatePublicURL(t *testing.T) {
	cases := []struct {
		label   string
		url     string
		wantErr bool
	}{
		// --- scheme validation ---
		{"ftp scheme", "ftp://8.8.8.8/path", true},
		{"file scheme", "file:///etc/passwd", true},
		{"no scheme (relative path)", "example.com/path", true},
		{"empty string", "", true},

		// --- host validation ---
		{"http with no host", "http://", true},

		// --- loopback ---
		{"ipv4 loopback 127.0.0.1", "http://127.0.0.1/path", true},
		{"ipv4 loopback 127.0.0.2", "http://127.0.0.2/path", true},
		{"ipv6 loopback ::1", "http://[::1]/path", true},

		// --- RFC 1918 ---
		{"rfc1918 10.0.0.1", "http://10.0.0.1/path", true},
		{"rfc1918 10.255.255.255", "http://10.255.255.255/path", true},
		{"rfc1918 172.16.0.1", "http://172.16.0.1/path", true},
		{"rfc1918 172.31.255.255", "http://172.31.255.255/path", true},
		{"rfc1918 192.168.0.1", "http://192.168.0.1/path", true},
		{"rfc1918 192.168.255.255", "http://192.168.255.255/path", true},

		// --- link-local ---
		{"link-local 169.254.0.1", "http://169.254.0.1/path", true},
		{"link-local 169.254.169.254", "http://169.254.169.254/path", true},

		// --- CGNAT ---
		{"cgnat 100.64.0.1", "http://100.64.0.1/path", true},
		{"cgnat 100.127.255.255", "http://100.127.255.255/path", true},

		// --- 0.0.0.0/8 ---
		{"0.0.0.0 block", "http://0.0.0.0/path", true},

		// --- IPv6 ULA / link-local ---
		{"ipv6 ula fc00::1", "http://[fc00::1]/path", true},
		{"ipv6 link-local fe80::1", "http://[fe80::1]/path", true},

		// --- public IPs (no DNS needed for numeric addresses) ---
		{"public http 8.8.8.8", "http://8.8.8.8/path", false},
		{"public https 8.8.8.8", "https://8.8.8.8/path", false},
		{"public 1.1.1.1", "https://1.1.1.1/", false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := ValidatePublicURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidatePublicURL(%q) = %v, wantErr = %v", tc.url, err, tc.wantErr)
			}
		})
	}
}
