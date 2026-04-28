//go:build !windows

package spotify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func TestIsInvalidGrant(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("network blip"), false},
		{"oauth invalid_grant", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}, true},
		{"oauth invalid_request", &oauth2.RetrieveError{ErrorCode: "invalid_request"}, false},
		{"wrapped invalid_grant", fmt.Errorf("refresh failed: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}), true},
		{"wrapped non-oauth", fmt.Errorf("refresh failed: %w", errors.New("transport error")), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInvalidGrant(tt.err)
			if got != tt.want {
				t.Errorf("isInvalidGrant(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestUsingFallbackToken(t *testing.T) {
	t.Run("no token source", func(t *testing.T) {
		s := &Session{}
		if !s.usingFallbackToken() {
			t.Error("usingFallbackToken() = false, want true with nil tokenSource")
		}
	})

	t.Run("with token source", func(t *testing.T) {
		conf := spotifyOAuthConfig("test-client-id")
		s := &Session{tokenSource: conf.TokenSource(t.Context(), &oauth2.Token{})}
		if s.usingFallbackToken() {
			t.Error("usingFallbackToken() = true, want false with non-nil tokenSource")
		}
	})
}

func TestDeleteCreds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("missing file", func(t *testing.T) {
		removed, err := DeleteCreds()
		if err != nil {
			t.Errorf("DeleteCreds() on missing file returned %v, want nil", err)
		}
		if removed {
			t.Error("DeleteCreds() reported removed=true for missing file")
		}
	})

	t.Run("removes existing file", func(t *testing.T) {
		dir := filepath.Join(home, ".config", "cliamp")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "spotify_credentials.json")
		if err := os.WriteFile(path, []byte(`{"username":"x"}`), 0o600); err != nil {
			t.Fatal(err)
		}

		removed, err := DeleteCreds()
		if err != nil {
			t.Fatalf("DeleteCreds() = %v, want nil", err)
		}
		if !removed {
			t.Error("DeleteCreds() reported removed=false after removing file")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file still exists after DeleteCreds: stat err = %v", err)
		}
	})
}

func TestCredsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := CredsPath()
	if err != nil {
		t.Fatalf("CredsPath() error = %v", err)
	}
	want := filepath.Join(home, ".config", "cliamp", "spotify_credentials.json")
	if got != want {
		t.Errorf("CredsPath() = %q, want %q", got, want)
	}
}
