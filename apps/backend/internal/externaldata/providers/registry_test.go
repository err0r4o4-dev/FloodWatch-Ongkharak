package providers

import "testing"

func TestRegistryDoesNotAcceptUnknownSources(t *testing.T) {
	if _, err := New([]string{"DDPM"}); err == nil {
		t.Fatal("unverified provider registered")
	}
	providers, err := New(nil)
	if err != nil || len(providers) != 0 {
		t.Fatal("default registry must be empty")
	}
}
