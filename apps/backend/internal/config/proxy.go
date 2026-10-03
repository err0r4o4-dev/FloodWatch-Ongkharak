package config

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
)

const maxTrustedProxyCIDRs = 32

func trustedProxyCIDRs() ([]netip.Prefix, error) {
	value := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if value == "" {
		return nil, nil
	}
	entries := strings.Split(value, ",")
	if len(entries) > maxTrustedProxyCIDRs {
		return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS must contain at most %d CIDRs", maxTrustedProxyCIDRs)
	}
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS contains an empty CIDR")
		}
		prefix, err := netip.ParsePrefix(entry)
		if err != nil || !prefix.IsValid() {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS contains an invalid CIDR")
		}
		if prefix.Bits() == 0 {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS must not contain an all-address /0 CIDR")
		}
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS must not contain mapped IPv6 CIDRs")
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}
