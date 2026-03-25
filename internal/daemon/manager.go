package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manager struct {
	homeDir string
}

type Status struct {
	Running   bool
	PID       int
	StartedAt time.Time
	LogPath   string
}

type metadata struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func New(homeDir string) *Manager {
	return &Manager{homeDir: homeDir}
}

func (m *Manager) Start(exePath string) (Status, error) {
	if err := m.ensureDir(); err != nil {
		return Status{}, err
	}
	status, _ := m.Status()
	if status.Running {
		return status, nil
	}

	logPath := m.logPath()
	pid, err := startDetached(exePath, []string{"__daemon", "serve"}, logPath)
	if err != nil {
		return Status{}, err
	}
	meta := metadata{
		PID:       pid,
		StartedAt: time.Now().UTC(),
	}
	if err := m.writeMeta(meta); err != nil {
		return Status{}, err
	}
	return Status{
		Running:   true,
		PID:       meta.PID,
		StartedAt: meta.StartedAt,
		LogPath:   logPath,
	}, nil
}

func (m *Manager) Stop() (Status, error) {
	meta, err := m.readMeta()
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Running: false, LogPath: m.logPath()}, nil
		}
		return Status{}, err
	}
	if err := terminateProcess(meta.PID); err != nil {
		return Status{}, err
	}
	_ = os.Remove(m.metaPath())
	return Status{
		Running:   false,
		PID:       meta.PID,
		StartedAt: meta.StartedAt,
		LogPath:   m.logPath(),
	}, nil
}

func (m *Manager) Restart(exePath string) (Status, error) {
	if _, err := m.Stop(); err != nil {
		return Status{}, err
	}
	return m.Start(exePath)
}

func (m *Manager) Status() (Status, error) {
	meta, err := m.readMeta()
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Running: false, LogPath: m.logPath()}, nil
		}
		return Status{}, err
	}
	running, err := isProcessRunning(meta.PID)
	if err != nil {
		return Status{}, err
	}
	if !running {
		_ = os.Remove(m.metaPath())
		return Status{Running: false, PID: meta.PID, StartedAt: meta.StartedAt, LogPath: m.logPath()}, nil
	}
	return Status{
		Running:   true,
		PID:       meta.PID,
		StartedAt: meta.StartedAt,
		LogPath:   m.logPath(),
	}, nil
}

func (m *Manager) ensureDir() error {
	return os.MkdirAll(m.daemonDir(), 0o755)
}

func (m *Manager) daemonDir() string {
	return filepath.Join(m.homeDir, ".qp", "daemon")
}

func (m *Manager) logPath() string {
	return filepath.Join(m.daemonDir(), "daemon.log")
}

func (m *Manager) metaPath() string {
	return filepath.Join(m.daemonDir(), "daemon.json")
}

func (m *Manager) readMeta() (metadata, error) {
	raw, err := os.ReadFile(m.metaPath())
	if err != nil {
		return metadata{}, err
	}
	var meta metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return metadata{}, fmt.Errorf("read daemon metadata: %w", err)
	}
	return meta, nil
}

func (m *Manager) writeMeta(meta metadata) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.metaPath(), raw, 0o644)
}
