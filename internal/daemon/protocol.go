package daemon

const (
	BypassEnvVar = "QP_DAEMON_BYPASS"
	pipeName     = `\\.\pipe\qp-daemon`
)

type executeRequest struct {
	Args []string `json:"args"`
	Cwd  string   `json:"cwd,omitempty"`
}

type executeEvent struct {
	Stream   string `json:"stream,omitempty"`
	Data     string `json:"data,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Done     bool   `json:"done,omitempty"`
	Error    string `json:"error,omitempty"`
}
