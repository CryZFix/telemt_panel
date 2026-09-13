package main

import (
	"testing"
	"time"

	"github.com/amirotin/telemt_panel/internal/config"
	"github.com/amirotin/telemt_panel/internal/store"
)

func TestAuthOptOutPreservesCredentials(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), Auth: config.AuthConfig{Username: "admin", PasswordHash: "saved-hash"}, Store: config.StoreConfig{Driver: "memory"}}
	s, err := newSourceStore(&config.Source{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreateWebAuthnUserHandle(make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWebAuthnCredential(store.WebAuthnCredential{ID: "test-id", Name: "Existing passkey", CredentialData: []byte(`{}`), Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Auth = config.AuthConfig{Disabled: true}
	s, err = newSourceStore(&config.Source{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := s.ListWebAuthnCredentials()
	if err != nil || len(keys) != 1 {
		t.Fatal("opt-out removed credentials", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Auth = config.AuthConfig{Username: "admin", PasswordHash: "saved-hash"}
	s, err = newSourceStore(&config.Source{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	keys, err = s.ListWebAuthnCredentials()
	if err != nil || len(keys) != 1 {
		t.Fatal("re-enable removed credentials", err)
	}
}
