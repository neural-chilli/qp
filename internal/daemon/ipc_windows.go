//go:build windows

package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
)

func Serve(exePath string, errOut io.Writer) error {
	l, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go handleConn(exePath, conn, errOut)
	}
}

func Proxy(args []string, cwd string, stdout, stderr io.Writer) (int, error) {
	timeout := 2 * time.Second
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(executeRequest{Args: args, Cwd: cwd}); err != nil {
		return 0, err
	}

	for {
		var evt executeEvent
		if err := dec.Decode(&evt); err != nil {
			return 0, err
		}
		if evt.Error != "" {
			return evt.ExitCode, fmt.Errorf(evt.Error)
		}
		switch evt.Stream {
		case "stdout":
			if _, err := io.WriteString(stdout, evt.Data); err != nil {
				return 0, err
			}
		case "stderr":
			if _, err := io.WriteString(stderr, evt.Data); err != nil {
				return 0, err
			}
		}
		if evt.Done {
			return evt.ExitCode, nil
		}
	}
}

func handleConn(exePath string, conn io.ReadWriteCloser, errOut io.Writer) {
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	var req executeRequest
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(executeEvent{Done: true, ExitCode: 1, Error: "invalid request: " + err.Error()})
		return
	}
	if len(req.Args) == 0 {
		_ = enc.Encode(executeEvent{Done: true, ExitCode: 1, Error: "empty args"})
		return
	}

	cmd := exec.Command(exePath, req.Args...)
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}
	cmd.Env = append(os.Environ(), BypassEnvVar+"=1")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = enc.Encode(executeEvent{Done: true, ExitCode: 1, Error: err.Error()})
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = enc.Encode(executeEvent{Done: true, ExitCode: 1, Error: err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		_ = enc.Encode(executeEvent{Done: true, ExitCode: 1, Error: err.Error()})
		return
	}

	var writeMu sync.Mutex
	stream := func(name string, reader io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				writeMu.Lock()
				_ = enc.Encode(executeEvent{Stream: name, Data: string(buf[:n])})
				writeMu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		stream("stdout", stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		stream("stderr", stderrPipe)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
		} else {
			exitCode = 1
			fmt.Fprintf(errOut, "daemon child wait error: %v\n", waitErr)
		}
	}

	writeMu.Lock()
	_ = enc.Encode(executeEvent{Done: true, ExitCode: exitCode})
	writeMu.Unlock()
}
