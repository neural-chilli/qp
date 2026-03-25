package config

import (
	"fmt"
	"strings"
	"unicode"
)

type RunExpr interface {
	isRunExpr()
}

type RunRef struct {
	Name string
}

func (RunRef) isRunExpr() {}

type RunSeq struct {
	Nodes []RunExpr
}

func (RunSeq) isRunExpr() {}

type RunPar struct {
	Nodes []RunExpr
}

func (RunPar) isRunExpr() {}

type RunWhen struct {
	Expr  string
	True  RunExpr
	False RunExpr
}

func (RunWhen) isRunExpr() {}

type RunSwitchCase struct {
	Value string
	Expr  RunExpr
}

type RunSwitch struct {
	Expr  string
	Cases []RunSwitchCase
}

func (RunSwitch) isRunExpr() {}

func ParseRunExpr(input string) (RunExpr, error) {
	p := &runParser{input: input}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected token near %q", p.input[p.pos:])
	}
	return expr, nil
}

func RunExprRefs(expr RunExpr) []string {
	seen := map[string]bool{}
	var refs []string
	var visit func(RunExpr)
	visit = func(node RunExpr) {
		switch n := node.(type) {
		case RunRef:
			if !seen[n.Name] {
				seen[n.Name] = true
				refs = append(refs, n.Name)
			}
		case RunSeq:
			for _, child := range n.Nodes {
				visit(child)
			}
		case RunPar:
			for _, child := range n.Nodes {
				visit(child)
			}
		case RunWhen:
			if n.True != nil {
				visit(n.True)
			}
			if n.False != nil {
				visit(n.False)
			}
		case RunSwitch:
			for _, c := range n.Cases {
				visit(c.Expr)
			}
		}
	}
	visit(expr)
	return refs
}

type runParser struct {
	input string
	pos   int
}

func (p *runParser) parseExpr() (RunExpr, error) {
	first, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	nodes := []RunExpr{first}
	for {
		p.skipSpace()
		if !p.consume("->") {
			break
		}
		next, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, next)
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return RunSeq{Nodes: nodes}, nil
}

func (p *runParser) parseTerm() (RunExpr, error) {
	p.skipSpace()
	if p.consumeParKeyword() {
		p.skipSpace()
		if !p.consume("(") {
			return nil, fmt.Errorf("expected '(' after par")
		}
		nodes := []RunExpr{}
		for {
			p.skipSpace()
			if p.consume(")") {
				break
			}
			node, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			p.skipSpace()
			if p.consume(")") {
				break
			}
			if !p.consume(",") {
				return nil, fmt.Errorf("expected ',' or ')' in par()")
			}
		}
		if len(nodes) == 0 {
			return nil, fmt.Errorf("par() requires at least one task")
		}
		return RunPar{Nodes: nodes}, nil
	}
	if p.consumeWhenKeyword() {
		p.skipSpace()
		if !p.consume("(") {
			return nil, fmt.Errorf("expected '(' after when")
		}
		cond, err := p.parseConditionArg()
		if err != nil {
			return nil, err
		}
		if !p.consume(",") {
			return nil, fmt.Errorf("when() requires condition and true branch")
		}
		trueExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		var falseExpr RunExpr
		p.skipSpace()
		if p.consume(",") {
			falseExpr, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		p.skipSpace()
		if !p.consume(")") {
			return nil, fmt.Errorf("expected ')' to close when()")
		}
		return RunWhen{Expr: cond, True: trueExpr, False: falseExpr}, nil
	}
	if p.consumeSwitchKeyword() {
		p.skipSpace()
		if !p.consume("(") {
			return nil, fmt.Errorf("expected '(' after switch")
		}
		cond, err := p.parseConditionArg()
		if err != nil {
			return nil, err
		}
		if !p.consume(",") {
			return nil, fmt.Errorf("switch() requires condition and at least one case")
		}
		cases := []RunSwitchCase{}
		for {
			p.skipSpace()
			if p.consume(")") {
				break
			}
			value, err := p.parseStringLiteral()
			if err != nil {
				return nil, fmt.Errorf("switch() case value: %w", err)
			}
			p.skipSpace()
			if !p.consume(":") {
				return nil, fmt.Errorf("switch() case %q: expected ':'", value)
			}
			caseExpr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			cases = append(cases, RunSwitchCase{Value: value, Expr: caseExpr})
			p.skipSpace()
			if p.consume(")") {
				break
			}
			if !p.consume(",") {
				return nil, fmt.Errorf("expected ',' or ')' in switch()")
			}
		}
		if len(cases) == 0 {
			return nil, fmt.Errorf("switch() requires at least one case")
		}
		return RunSwitch{Expr: cond, Cases: cases}, nil
	}

	name := p.parseIdentifier()
	if name == "" {
		return nil, fmt.Errorf("expected task reference")
	}
	return RunRef{Name: name}, nil
}

func (p *runParser) consumeParKeyword() bool {
	p.skipSpace()
	if !strings.HasPrefix(p.input[p.pos:], "par") {
		return false
	}
	next := p.pos + len("par")
	if next < len(p.input) {
		r := rune(p.input[next])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			return false
		}
	}
	p.pos = next
	return true
}

func (p *runParser) consumeWhenKeyword() bool {
	p.skipSpace()
	if !strings.HasPrefix(p.input[p.pos:], "when") {
		return false
	}
	next := p.pos + len("when")
	if next < len(p.input) {
		r := rune(p.input[next])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			return false
		}
	}
	p.pos = next
	return true
}

func (p *runParser) consumeSwitchKeyword() bool {
	p.skipSpace()
	if !strings.HasPrefix(p.input[p.pos:], "switch") {
		return false
	}
	next := p.pos + len("switch")
	if next < len(p.input) {
		r := rune(p.input[next])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			return false
		}
	}
	p.pos = next
	return true
}

func (p *runParser) parseConditionArg() (string, error) {
	p.skipSpace()
	start := p.pos
	depth := 0
	var quote rune
	escaped := false

	for p.pos < len(p.input) {
		ch := rune(p.input[p.pos])
		if quote != 0 {
			if escaped {
				escaped = false
				p.pos++
				continue
			}
			if ch == '\\' {
				escaped = true
				p.pos++
				continue
			}
			if ch == quote {
				quote = 0
			}
			p.pos++
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return "", fmt.Errorf("when() requires a true branch after condition")
			}
			depth--
		case ',':
			if depth == 0 {
				cond := strings.TrimSpace(p.input[start:p.pos])
				if cond == "" {
					return "", fmt.Errorf("when() condition cannot be empty")
				}
				return cond, nil
			}
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated when() condition")
}

func (p *runParser) parseIdentifier() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return ""
	}
	return p.input[start:p.pos]
}

func (p *runParser) parseStringLiteral() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return "", fmt.Errorf("expected quoted string")
	}
	quote := p.input[p.pos]
	if quote != '"' && quote != '\'' {
		return "", fmt.Errorf("expected quoted string")
	}
	p.pos++
	var out strings.Builder
	escaped := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		p.pos++
		if escaped {
			out.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == quote {
			return out.String(), nil
		}
		out.WriteByte(ch)
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (p *runParser) skipSpace() {
	for p.pos < len(p.input) {
		if p.input[p.pos] != ' ' && p.input[p.pos] != '\t' && p.input[p.pos] != '\n' && p.input[p.pos] != '\r' {
			return
		}
		p.pos++
	}
}

func (p *runParser) consume(token string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.input[p.pos:], token) {
		p.pos += len(token)
		return true
	}
	return false
}
