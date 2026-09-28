package config

import "testing"

func TestResolveAPIKeyConfiguredEnvOnly(t *testing.T) {
	t.Setenv("MY_KEY", "custom")
	t.Setenv("OPENAI_API_KEY", "openai")
	t.Setenv("GOMMIT_API_KEY", "fallback")

	got, err := ResolveAPIKey("openai", "MY_KEY")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if got != "custom" {
		t.Fatalf("got %q, want %q", got, "custom")
	}
}

func TestResolveAPIKeyConfiguredEnvMissing(t *testing.T) {
	t.Setenv("MY_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai")

	if _, err := ResolveAPIKey("openai", "MY_KEY"); err == nil {
		t.Fatal("expected error when configured env var is empty")
	}
}

func TestResolveAPIKeyFallbackChain(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOMMIT_API_KEY", "fallback")

	got, err := ResolveAPIKey("openai", "")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if got != "fallback" {
		t.Fatalf("got %q, want %q", got, "fallback")
	}

	t.Setenv("OPENAI_API_KEY", "openai")
	if got, err = ResolveAPIKey("openai", ""); err != nil || got != "openai" {
		t.Fatalf("got %q, %v; want %q", got, err, "openai")
	}
}
