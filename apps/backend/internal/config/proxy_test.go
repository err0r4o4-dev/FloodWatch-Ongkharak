package config

import (
	"strings"
	"testing"
)

func TestLoadDefaultsTrustedProxyCIDRsToNone(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(configuration.TrustedProxyCIDRs) != 0 {
		t.Fatalf("TrustedProxyCIDRs = %v, want none", configuration.TrustedProxyCIDRs)
	}
}

func TestLoadParsesAndMasksTrustedProxyCIDRs(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", " 192.0.2.10/24, 2001:db8:1::42/48 ")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := configuration.TrustedProxyCIDRs; len(got) != 2 || got[0].String() != "192.0.2.0/24" || got[1].String() != "2001:db8:1::/48" {
		t.Fatalf("TrustedProxyCIDRs = %v, want masked IPv4 and IPv6 prefixes", got)
	}
}

func TestLoadRejectsUnsafeTrustedProxyCIDRs(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "invalid", value: "not-a-cidr"},
		{name: "all IPv4", value: "0.0.0.0/0"},
		{name: "all IPv6", value: "::/0"},
		{name: "mapped IPv6", value: "::ffff:192.0.2.0/120"},
		{name: "empty entry", value: "192.0.2.0/24,"},
		{name: "too many", value: strings.Repeat("192.0.2.0/24,", maxTrustedProxyCIDRs) + "192.0.2.1/32"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv("TRUSTED_PROXY_CIDRS", test.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
				t.Fatalf("Load() error = %v, want TRUSTED_PROXY_CIDRS validation error", err)
			}
		})
	}
}
