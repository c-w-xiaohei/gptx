//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
)

func configureJobWorkerProcess(cmd *exec.Cmd) {}

func signalJobProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find job process: %w", err)
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("kill job process: %w", err)
	}
	return nil
}
