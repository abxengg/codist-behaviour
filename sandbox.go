package behaviour

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The project's code runs in two places: when the harness calls its methods, and when Maven builds it (a build can
// run plugins). Both are confined by the operating system where it offers a way: sandbox-exec on macOS, bwrap on
// Linux. Confined code can write only in Codist's work folder (and, for a build, the Maven cache), cannot read the
// rest of the home folder, and the harness has no network; a build keeps it, since Maven may have to download.
// Where there is no way to confine (Windows, a Linux without bwrap), the code runs as before and the report says so.

// confine describes one confined command.
type confine struct {
	work    string // Codist's work folder: readable and writable
	network bool   // whether the command may reach the network
	maven   bool   // whether the Maven cache may be written
}

// Confined says how runs are confined on this machine, or "" when they are not.
func Confined() string {
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat("/usr/bin/sandbox-exec"); err == nil {
			return "sandbox-exec"
		}
	case "linux":
		if p, err := exec.LookPath("bwrap"); err == nil {
			// bwrap is there but may not be allowed to make a sandbox (some containers): try it once.
			if exec.Command(p, "--ro-bind", "/", "/", "true").Run() == nil {
				return "bwrap"
			}
		}
	}
	return ""
}

var confinedBy = Confined()

// command makes the command, confined when the machine can do it.
func (c confine) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	home, _ := os.UserHomeDir()
	real := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	work := real(c.work)
	m2 := filepath.Join(home, ".m2")
	// What the command needs to read inside the home folder: the JDK and Maven when they are installed there (as
	// SDKMAN and similar tools do), and the Maven cache with the project's libraries.
	reads := []string{m2}
	for _, bin := range []string{name, os.Getenv("JAVA_HOME"), lookPath("java"), lookPath("mvn")} {
		if bin == "" {
			continue
		}
		p := real(bin)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			p = filepath.Dir(filepath.Dir(p)) // …/bin/java -> the JDK's folder
		}
		if home != "" && strings.HasPrefix(p, real(home)+string(filepath.Separator)) && !strings.HasPrefix(p, work) {
			reads = append(reads, p)
		}
	}
	switch confinedBy {
	case "sandbox-exec":
		var b strings.Builder
		q := func(p string) string { return `"` + strings.ReplaceAll(real(p), `"`, `\"`) + `"` }
		b.WriteString("(version 1)\n(allow default)\n")
		if !c.network {
			b.WriteString("(deny network*)\n")
		}
		b.WriteString("(deny file-write* (subpath \"/\"))\n")
		fmt.Fprintf(&b, "(allow file-write* (subpath %s) (literal \"/dev/null\") (literal \"/dev/tty\") (literal \"/dev/dtracehelper\"))\n", q(work))
		if c.maven {
			fmt.Fprintf(&b, "(allow file-write* (subpath %s))\n", q(m2))
		}
		if home != "" {
			fmt.Fprintf(&b, "(deny file-read* (subpath %s))\n(allow file-read-metadata)\n(allow file-read* (subpath %s)", q(home), q(work))
			for _, p := range reads {
				fmt.Fprintf(&b, " (subpath %s)", q(p))
			}
			b.WriteString(")\n")
		}
		return exec.CommandContext(ctx, "/usr/bin/sandbox-exec", append([]string{"-p", b.String(), name}, args...)...)
	case "bwrap":
		a := []string{"--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--die-with-parent"}
		if home != "" {
			a = append(a, "--tmpfs", home) // the home folder is empty, except for what is put back below
			for _, p := range reads {
				if _, err := os.Stat(p); err == nil {
					a = append(a, "--ro-bind", p, p)
				}
			}
		}
		a = append(a, "--bind", work, work)
		if c.maven {
			if os.MkdirAll(m2, 0o755) == nil {
				a = append(a, "--bind", m2, m2)
			}
		}
		if !c.network {
			a = append(a, "--unshare-net")
		}
		return exec.CommandContext(ctx, "bwrap", append(append(a, name), args...)...)
	}
	return exec.CommandContext(ctx, name, args...)
}

func lookPath(name string) string {
	p, _ := exec.LookPath(name)
	return p
}

// unconfinedNote is what a report says when the project's code ran without a sandbox.
const unconfinedNote = "This machine has no sandbox Codist can use (sandbox-exec on macOS, bwrap on Linux), so the code ran as you, with your files in reach."
