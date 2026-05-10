//go:build !windows

package cli

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureJobWorkerProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func signalJobProcess(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal job process: %w", err)
	}
	return nil
}
