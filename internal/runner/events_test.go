package runner

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEventStreamEmitsIteration(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	stream := NewEventStream(&out)
	stream.EmitIteration("test", 2, 5, "pass")

	var event map[string]any
	if err := json.Unmarshal(out.Bytes(), &event); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got := event["type"]; got != "iteration" {
		t.Fatalf("type = %v, want iteration", got)
	}
	if got := event["task"]; got != "test" {
		t.Fatalf("task = %v, want test", got)
	}
	if got := event["iteration"]; got != float64(2) {
		t.Fatalf("iteration = %v, want 2", got)
	}
	if got := event["max"]; got != float64(5) {
		t.Fatalf("max = %v, want 5", got)
	}
	if got := event["status"]; got != "pass" {
		t.Fatalf("status = %v, want pass", got)
	}
}

func TestEventStreamEmitsApprovalRequired(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	stream := NewEventStream(&out)
	stream.EmitApprovalRequired("deploy", "destructive step", map[string]any{
		"gate":  "manual",
		"level": "high",
		"type":  "ignored",
	})

	var event map[string]any
	if err := json.Unmarshal(out.Bytes(), &event); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got := event["type"]; got != "approval_required" {
		t.Fatalf("type = %v, want approval_required", got)
	}
	if got := event["task"]; got != "deploy" {
		t.Fatalf("task = %v, want deploy", got)
	}
	if got := event["reason"]; got != "destructive step" {
		t.Fatalf("reason = %v, want destructive step", got)
	}
	if got := event["gate"]; got != "manual" {
		t.Fatalf("gate = %v, want manual", got)
	}
	if got := event["level"]; got != "high" {
		t.Fatalf("level = %v, want high", got)
	}
}
