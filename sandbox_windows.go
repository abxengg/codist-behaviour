//go:build windows

package behaviour

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// On Windows the project's code runs at low integrity, the level browsers use for web content. Windows then refuses
// its writes anywhere not labelled for low integrity: Codist labels only its work folder. It is a partial sandbox:
// the code can still read your files and use the network, and each report says so.

// lowToken is a copy of this process's token at low integrity.
func lowToken() (windows.Token, error) {
	var own windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ADJUST_DEFAULT|windows.TOKEN_ASSIGN_PRIMARY, &own); err != nil {
		return 0, err
	}
	defer own.Close()
	var low windows.Token
	if err := windows.DuplicateTokenEx(own, 0, nil, windows.SecurityImpersonation, windows.TokenPrimary, &low); err != nil {
		return 0, err
	}
	sid, err := windows.StringToSid("S-1-16-4096") // Mandatory Label\\Low Mandatory Level
	if err != nil {
		low.Close()
		return 0, err
	}
	tml := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: sid, Attributes: windows.SE_GROUP_INTEGRITY}}
	if err := windows.SetTokenInformation(low, windows.TokenIntegrityLevel, (*byte)(unsafePointer(&tml)), tml.Size()); err != nil {
		low.Close()
		return 0, err
	}
	return low, nil
}

// confineWindows runs cmd at low integrity, after labelling the work folder so the code may write there. It says
// whether that worked; when it did not, the command runs as it was.
func confineWindows(cmd *exec.Cmd, work string) bool {
	if err := os.MkdirAll(work, 0o755); err != nil {
		return false
	}
	// icacls is part of Windows: the label is inherited by everything made in the folder from now on, and /T
	// applies it to what is already there.
	if exec.Command("icacls", work, "/setintegritylevel", "(OI)(CI)low", "/T", "/C", "/Q").Run() != nil {
		return false
	}
	tok, err := lowToken()
	if err != nil {
		return false
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Token: syscall.Token(tok)}
	return true
}

func windowsConfinable() bool {
	tok, err := lowToken()
	if err != nil {
		return false
	}
	tok.Close()
	return true
}
