package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var ErrPrivateForwardURL = errors.New("forward_url points to a private or local network address")

func ValidateForwardURL(raw string, allowPrivateNetworks bool) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("forward_url is empty")
	}
	if len(trimmed) > 2000 {
		return errors.New("forward_url must be shorter than 2000 characters")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("forward_url is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("forward_url must start with http:// or https://")
	}
	if parsed.User != nil {
		return errors.New("forward_url must not contain user info")
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return errors.New("forward_url host is required")
	}
	if isLocalHostname(host) && !allowPrivateNetworks {
		return ErrPrivateForwardURL
	}
	if ip, err := netip.ParseAddr(host); err == nil && isPrivateNetworkAddress(ip) && !allowPrivateNetworks {
		return ErrPrivateForwardURL
	}

	return nil
}

func ValidateResolvedForwardHost(ctx context.Context, host string, allowPrivateNetworks bool) error {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return errors.New("forward_url host is required")
	}
	if allowPrivateNetworks {
		return nil
	}
	if isLocalHostname(trimmed) {
		return ErrPrivateForwardURL
	}
	if ip, err := netip.ParseAddr(trimmed); err == nil {
		if isPrivateNetworkAddress(ip) {
			return ErrPrivateForwardURL
		}
		return nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, trimmed)
	if err != nil {
		return fmt.Errorf("resolve forward_url host failed: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("resolve forward_url host returned no addresses")
	}
	for _, item := range ips {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return fmt.Errorf("resolve forward_url host returned invalid address: %s", item.IP.String())
		}
		if isPrivateNetworkAddress(addr) {
			return ErrPrivateForwardURL
		}
	}
	return nil
}

func isLocalHostname(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") || strings.HasSuffix(normalized, ".local")
}

func isPrivateNetworkAddress(addr netip.Addr) bool {
	return addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsUnspecified() ||
		addr.IsMulticast()
}
