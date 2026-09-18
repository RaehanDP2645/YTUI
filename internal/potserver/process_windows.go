//go:build windows

package potserver

import (
	"os/exec"
	"syscall"
)

// configureProcess menyembunyikan jendela proses Node di Windows
// (CREATE_NO_WINDOW), jadi tidak ada CMD/terminal yang muncul.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
