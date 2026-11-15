//go:build windows

package behaviour

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// At low integrity the code can write in the work folder and nowhere in the home folder.
func TestConfinedWindows(t *testing.T) {
	if confinedBy != "low integrity" {
		if os.Getenv("CODIST_REQUIRE_SANDBOX") != "" {
			t.Fatal("low integrity could not be used")
		}
		t.Skip("low integrity could not be used here")
	}
	home, _ := os.UserHomeDir()
	outside := filepath.Join(home, "codist-sandbox-test-written.txt")
	defer os.Remove(outside)
	work := t.TempDir()
	inside := filepath.Join(work, "f.txt")
	run := func(target string) {
		cmd := confine{work: work}.command(context.Background(), "cmd", "/c", "echo x>"+target)
		_ = cmd.Run()
	}
	run(inside)
	if _, err := os.Stat(inside); err != nil {
		t.Errorf("the work folder is writable: %v", err)
	}
	run(outside)
	if _, err := os.Stat(outside); err == nil {
		t.Error("a file was written in the home folder")
	}
}
