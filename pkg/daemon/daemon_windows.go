//go:build windows
// +build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func Daemonize() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	args := make([]string, 0)
	for _, arg := range os.Args[1:] {
		if arg == "-daemon" || arg == "--daemon" {
			continue
		}
		args = append(args, arg)
	}

	cmd := exec.Command(executable, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	cmd.Process.Release()
	return nil
}

