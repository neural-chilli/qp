package config

import "testing"

func TestParseRunExprParsesNestedGraph(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr("par(lint, test) -> build")
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"lint": true, "test": true, "build": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}

func TestParseRunExprRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := ParseRunExpr("par(lint test)")
	if err == nil {
		t.Fatal("ParseRunExpr() error = nil, want parse error")
	}
}

func TestParseRunExprWhenExpression(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr(`when(branch() == "main", deploy, notify)`)
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"deploy": true, "notify": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}

func TestParseRunExprSwitchExpression(t *testing.T) {
	t.Parallel()

	expr, err := ParseRunExpr(`switch(env("TARGET"), "api": build-api -> deploy-api, "web": build-web)`)
	if err != nil {
		t.Fatalf("ParseRunExpr() error = %v", err)
	}
	refs := RunExprRefs(expr)
	want := map[string]bool{"build-api": true, "deploy-api": true, "build-web": true}
	if len(refs) != len(want) {
		t.Fatalf("RunExprRefs() = %#v, want %#v", refs, want)
	}
	for _, ref := range refs {
		if !want[ref] {
			t.Fatalf("unexpected ref %q in %#v", ref, refs)
		}
	}
}
