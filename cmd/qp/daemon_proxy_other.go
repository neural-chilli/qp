//go:build !windows

package main

import "os"

func maybeProxyToDaemon(args []string, stdout, stderr *os.File) (int, bool) {
	_, _, _ = args, stdout, stderr
	return 0, false
}
