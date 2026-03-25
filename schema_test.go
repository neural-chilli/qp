package qp

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSchemaParsesAndIncludesCoreFields(t *testing.T) {
	raw, err := os.ReadFile("qp.schema.json")
	if err != nil {
		t.Fatalf("ReadFile(qp.schema.json) error = %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("json.Unmarshal(qp.schema.json) error = %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema.properties = %#v, want object", schema["properties"])
	}
	if _, ok := properties["tasks"]; !ok {
		t.Fatalf("schema.properties = %#v, want tasks field", properties)
	}
	for _, key := range []string{"vars", "templates", "profiles"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("schema.properties = %#v, want %s field", properties, key)
		}
	}

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema.$defs = %#v, want object", schema["$defs"])
	}
	task, ok := defs["task"].(map[string]any)
	if !ok {
		t.Fatalf("schema.$defs.task = %#v, want object", defs["task"])
	}
	taskProps, ok := task["properties"].(map[string]any)
	if !ok {
		t.Fatalf("task.properties = %#v, want object", task["properties"])
	}
	if _, ok := taskProps["safety"]; !ok {
		t.Fatalf("task.properties = %#v, want safety field", taskProps)
	}
	if _, ok := taskProps["run"]; !ok {
		t.Fatalf("task.properties = %#v, want run field", taskProps)
	}
}
