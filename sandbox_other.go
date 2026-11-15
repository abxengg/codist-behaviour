//go:build !windows

package behaviour

import "os/exec"

func confineWindows(*exec.Cmd, string) bool { return false }

func windowsConfinable() bool { return false }
