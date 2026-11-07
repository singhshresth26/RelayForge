package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
)

var (
	ErrDisallowedScheme = errors.New("webhook URL must use https or http scheme")
	ErrPrivateIPTarget  = errors.New("destination resolves to private, loopback, or metadata IP address")
	ErrEmptyHost        = errors.New("destination URL host cannot be empty")
)

var privateIPBlocks []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",          // IPv4 loopback
		"::1/128",              // IPv6 loopback
		"10.0.0.0/8",           // RFC 1918 class A
		"172.16.0.0/12",        // RFC 1918 class B
		"192.168.0.0/16",       // RFC 1918 class C
		"169.254.0.0/16",       // IPv4 link-local (AWS/GCP metadata: 169.254.169.254)
		"100.64.0.0/10",        // Carrier-grade NAT
		"192.0.0.0/24",         // IETF Protocol Assignments
		"192.0.2.0/24",         // TEST-NET-1
		"198.18.0.0/15",        // Network benchmark tests
		"198.51.100.0/24",      // TEST-NET-2
		"203.0.113.0/24",       // TEST-NET-3
		"224.0.0.0/4",          // Multicast
		"240.0.0.0/4",          // Reserved
		"fc00::/7",             // IPv6 Unique Local Address
		"fe80::/10",            // IPv6 Link Local
		"ff00::/8",             // IPv6 Multicast
	}

	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// IsPrivateIP checks if an IP belongs to any private, loopback, or link-local range.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

type SSRFValidator struct {
	AllowPrivateIPs bool // Allowed for test environments or local testing
	EnforceHTTPS    bool // Enforces HTTPS in production
}

func NewSSRFValidator(allowPrivateIPs, enforceHTTPS bool) *SSRFValidator {
	return &SSRFValidator{
		AllowPrivateIPs: allowPrivateIPs,
		EnforceHTTPS:    enforceHTTPS,
	}
}

// ValidateURL performs static and DNS verification of the destination URL.
func (v *SSRFValidator) ValidateURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if v.EnforceHTTPS {
		if scheme != "https" {
			return errors.New("webhook destination URL must use HTTPS in production")
		}
	} else {
		if scheme != "http" && scheme != "https" {
			return ErrDisallowedScheme
		}
	}

	host := parsed.Hostname()
	if host == "" {
		return ErrEmptyHost
	}

	// If private IPs are allowed (e.g. in test suites), skip IP block checks
	if v.AllowPrivateIPs {
		return nil
	}

	// Resolve IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no IP address found for %s", host)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return fmt.Errorf("%w: %s resolves to %s", ErrPrivateIPTarget, host, ip.String())
		}
	}

	return nil
}

// SafeDialerControl validates that the socket IP connection being established is not private.
// This prevents DNS rebinding (TOCTOU) attacks where DNS changes between validation and dial.
func (v *SSRFValidator) SafeDialerControl() func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		if v.AllowPrivateIPs {
			return nil
		}

		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}

		ip := net.ParseIP(host)
		if ip != nil && IsPrivateIP(ip) {
			return fmt.Errorf("%w: connection to %s rejected", ErrPrivateIPTarget, ip.String())
		}

		return nil
	}
}

// SafeDialContext returns a net.DialContext function hardened against SSRF and DNS rebinding.
func (v *SSRFValidator) SafeDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Control: v.SafeDialerControl(),
	}
	return dialer.DialContext
}

