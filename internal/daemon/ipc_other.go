//go:build !windows

package daemon

import (
	"fmt"
	"io"
)

func Serve(exePath string, errOut io.Writer) error {
	_, _ = exePath, errOut
	return fmt.Errorf("daemon IPC is only supported on Windows")
}

func Proxy(args []string, cwd string, stdout, stderr io.Writer) (int, error) {
	_, _, _, _ = args, cwd, stdout, stderr
	return 0, fmt.Errorf("daemon IPC is only supported on Windows")
}
