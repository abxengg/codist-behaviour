package behaviour

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Confined code can write in the work folder and nowhere else, cannot read the home folder, and has no network.
func TestConfined(t *testing.T) {
	if confinedBy == "" || runtime.GOOS == "windows" {
		if os.Getenv("CODIST_REQUIRE_SANDBOX") != "" { // set where a sandbox is installed on purpose, as in CI on Linux
			t.Fatal("no sandbox could be used on this machine")
		}
		t.Skip("no sandbox on this machine")
	}
	home, _ := os.UserHomeDir()
	secret, err := os.CreateTemp(home, ".codist-sandbox-test-*")
	if err != nil {
		t.Skip("cannot write a file in the home folder")
	}
	secret.WriteString("secret")
	secret.Close()
	defer os.Remove(secret.Name())
	work := t.TempDir()
	outside := filepath.Join(home, ".codist-sandbox-test-written")
	defer os.Remove(outside)
	sh := func(c confine, script string) (string, error) {
		out, err := c.command(context.Background(), "/bin/sh", "-c", script).CombinedOutput()
		return string(out), err
	}
	c := confine{work: work}
	if out, err := sh(c, "echo ok > "+filepath.Join(work, "f")+" && cat "+filepath.Join(work, "f")); err != nil || strings.TrimSpace(out) != "ok" {
		t.Errorf("the work folder is writable: %q %v", out, err)
	}
	if out, err := sh(c, "cat "+secret.Name()); err == nil || strings.Contains(out, "secret") {
		t.Errorf("a file in the home folder was read: %q", out)
	}
	if _, err := sh(c, "echo x > "+outside); err == nil {
		t.Error("a file was written outside the work folder")
	}
	if _, err := os.Stat(outside); err == nil {
		t.Error("the file outside the work folder exists")
	}
	if _, err := os.Stat("/usr/bin/nc"); err == nil {
		if _, err := sh(c, "nc -z -w 2 1.1.1.1 53"); err == nil {
			t.Error("confined code reached the network")
		}
	}
}
