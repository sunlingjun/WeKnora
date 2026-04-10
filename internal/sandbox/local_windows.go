//go:build windows

package sandbox

import "os/exec"

func setProcessGroup(cmd *exec.Cmd) {
	// Windows syscall.SysProcAttr has no Setpgid; rely on CommandContext and Process.Kill.
	_ = cmd
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
