package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAllIncludesBuiltinThemes(t *testing.T) {
	// Point HOME somewhere empty so only embedded themes load.
	t.Setenv("HOME", t.TempDir())

	themes := LoadAll()
	if len(themes) == 0 {
		t.Fatal("LoadAll() returned no themes, expected built-in set")
	}

	// Check a well-known theme is present (dracula ships with the project).
	var hasDracula bool
	for _, th := range themes {
		if th.Name == "dracula" {
			hasDracula = true
			if th.Accent == "" {
				t.Error("dracula theme has empty Accent — embed/parse failed")
			}
			break
		}
	}
	if !hasDracula {
		t.Error("built-in themes missing dracula")
	}
}

func TestLoadAllSortedCaseInsensitive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	themes := LoadAll()
	for i := 1; i < len(themes); i++ {
		a := strings.ToLower(themes[i-1].Name)
		b := strings.ToLower(themes[i].Name)
		if a > b {
			t.Errorf("themes not sorted: %q before %q", themes[i-1].Name, themes[i].Name)
		}
	}
}

func TestLoadAllUserThemeOverridesBuiltin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Put a user override file named "dracula.toml" with a distinctive accent color.
	userDir := filepath.Join(home, ".config", "cliamp", "themes")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	overridden := `accent = "#ff00ff"
fg = "#123456"
`
	if err := os.WriteFile(filepath.Join(userDir, "dracula.toml"), []byte(overridden), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	themes := LoadAll()
	var got Theme
	for _, th := range themes {
		if strings.EqualFold(th.Name, "dracula") {
			got = th
			break
		}
	}
	if got.Name == "" {
		t.Fatal("dracula theme not present after override")
	}
	if got.Accent != "#ff00ff" {
		t.Errorf("Accent = %q, want #ff00ff (user override)", got.Accent)
	}
	if got.FG != "#123456" {
		t.Errorf("FG = %q, want #123456 (user override)", got.FG)
	}
}

func TestLoadAllAddsUserOnlyTheme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userDir := filepath.Join(home, ".config", "cliamp", "themes")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	custom := `accent = "#abcdef"`
	if err := os.WriteFile(filepath.Join(userDir, "mytheme.toml"), []byte(custom), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	themes := LoadAll()
	var found bool
	for _, th := range themes {
		if th.Name == "mytheme" {
			found = true
			if th.Accent != "#abcdef" {
				t.Errorf("Accent = %q, want #abcdef", th.Accent)
			}
		}
	}
	if !found {
		t.Error("user theme mytheme not loaded")
	}
}

func TestLoadAllIgnoresNonTomlFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userDir := filepath.Join(home, ".config", "cliamp", "themes")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "notatheme.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Subdirectory should also be ignored.
	if err := os.MkdirAll(filepath.Join(userDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}

	themes := LoadAll()
	for _, th := range themes {
		if th.Name == "notatheme" || th.Name == "nested" {
			t.Errorf("non-toml entry %q leaked into LoadAll()", th.Name)
		}
	}
}

func TestLoadAllMissingUserDir(t *testing.T) {
	// HOME points at a dir where ~/.config/cliamp/themes doesn't exist.
	t.Setenv("HOME", t.TempDir())
	themes := LoadAll()
	if len(themes) == 0 {
		t.Error("LoadAll() with missing user dir should still return built-in themes")
	}
}
