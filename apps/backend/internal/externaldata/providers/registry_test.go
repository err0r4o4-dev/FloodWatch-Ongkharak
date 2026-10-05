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

func TestRegistrySupportsOptInFM91(t *testing.T) {
	registered, err := New([]string{"FM91_NEWS"})
	if err != nil || registered["FM91_NEWS"] == nil || len(registered) != 1 {
		t.Fatalf("FM91 registry=%v err=%v", registered, err)
	}
}
