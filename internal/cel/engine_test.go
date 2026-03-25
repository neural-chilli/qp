package cel

import "testing"

func TestEvalBoolReturnsTrue(t *testing.T) {
	t.Parallel()

	engine := New()
	ok, err := engine.EvalBool(`branch == "main" && retries < 3`, map[string]any{
		"branch":  "main",
		"retries": 2,
	})
	if err != nil {
		t.Fatalf("EvalBool() error = %v", err)
	}
	if !ok {
		t.Fatal("EvalBool() = false, want true")
	}
}

func TestEvalSupportsMapIndexing(t *testing.T) {
	t.Parallel()

	engine := New()
	value, err := engine.Eval(`meta["tier"] == "prod"`, map[string]any{
		"meta": map[string]any{"tier": "prod"},
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	boolean, ok := value.(bool)
	if !ok || !boolean {
		t.Fatalf("Eval() = %#v, want true", value)
	}
}

func TestEvalBoolRejectsNonBooleanResult(t *testing.T) {
	t.Parallel()

	engine := New()
	_, err := engine.EvalBool(`version + 1`, map[string]any{"version": 1})
	if err == nil {
		t.Fatal("EvalBool() error = nil, want non-bool error")
	}
}

func TestEvalReturnsParseErrors(t *testing.T) {
	t.Parallel()

	engine := New()
	_, err := engine.Eval(`branch ==`, map[string]any{"branch": "main"})
	if err == nil {
		t.Fatal("Eval() error = nil, want parse error")
	}
}

func TestEvalSupportsBranchFunctionWithoutStringRewrite(t *testing.T) {
	t.Parallel()

	engine := New()
	value, err := engine.Eval(`branch() == "main" && "branch()" == "branch()"`, map[string]any{
		"branch": "main",
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	boolean, ok := value.(bool)
	if !ok || !boolean {
		t.Fatalf("Eval() = %#v, want true", value)
	}
}

func TestEvalSupportsEnvFunctionWithoutStringRewrite(t *testing.T) {
	t.Parallel()

	engine := New()
	value, err := engine.Eval(`env("MODE") == "prod" && "env(\"MODE\")" == "env(\"MODE\")"`, map[string]any{
		"env": map[string]string{
			"MODE": "prod",
		},
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	boolean, ok := value.(bool)
	if !ok || !boolean {
		t.Fatalf("Eval() = %#v, want true", value)
	}
}

func TestEvalSupportsProfileFunction(t *testing.T) {
	t.Parallel()

	engine := New()
	value, err := engine.Eval(`profile() == "prod"`, map[string]any{
		"profile": "prod",
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	boolean, ok := value.(bool)
	if !ok || !boolean {
		t.Fatalf("Eval() = %#v, want true", value)
	}
}
