package util

import (
	"errors"
	"net"
	"net/url"
)

var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // link-local
		"100.64.0.0/10",  // CGNAT
		"0.0.0.0/8",
		"::1/128",         // IPv6 loopback
		"fc00::/7",        // IPv6 ULA
		"fe80::/10",       // IPv6 link-local
	}
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, network)
	}
}

// ValidatePublicURL verifica que a URL usa http/https e aponta para um IP público.
// Bloqueia SSRF contra endereços internos, loopback e ranges reservados.
func ValidatePublicURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("URL inválida")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL deve usar http ou https")
	}
	if u.Host == "" {
		return errors.New("URL deve ter um host")
	}

	hostname := u.Hostname()
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return errors.New("não foi possível resolver o host da URL")
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		for _, r := range privateRanges {
			if r.Contains(ip) {
				return errors.New("URL aponta para um endereço reservado ou privado")
			}
		}
	}
	return nil
}
