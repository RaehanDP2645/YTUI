//go:build !windows

package potserver

import "os/exec"

// configureProcess tidak melakukan apa-apa di platform non-Windows; tidak ada
// jendela konsol yang perlu disembunyikan di sana.
func configureProcess(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
