//go:build !windows

package main

import "fmt"

func installWindowsPowerShellShim() (string, error) {
	return "", fmt.Errorf("windows only")
}
