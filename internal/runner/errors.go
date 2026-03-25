package runner

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	genericErrorPattern  = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?:\s*(.+)$`)
	goTestPattern        = regexp.MustCompile(`^\s*(.+?\.go):(\d+):\s*(.+)$`)
	pytestPattern        = regexp.MustCompile(`^(.+?):(\d+):\s+(.+)$`)
	tscPattern           = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s+error\s+[^:]+:\s+(.+)$`)
	eslintSummaryPattern = regexp.MustCompile(`^\d+:\d+\s+error\s+.+$`)
)

func extractErrors(format, stderr string) []ErrorEntry {
	if stderr == "" || format == "" {
		return nil
	}
	var parsed []ErrorEntry
	switch format {
	case "go_test":
		parsed = parseGoTestErrors(stderr)
	case "pytest":
		parsed = parsePytestErrors(stderr)
	case "tsc":
		parsed = parseTscErrors(stderr)
	case "eslint":
		parsed = parseEslintErrors(stderr)
	case "generic":
		return parseGenericErrors(stderr)
	default:
		return nil
	}
	if len(parsed) == 0 {
		return parseGenericErrors(stderr)
	}
	return parsed
}

func parseGoTestErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := goTestPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Message:  strings.TrimSpace(match[3]),
			Severity: "error",
		})
	}
	return errors
}

func parsePytestErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := pytestPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Message:  strings.TrimSpace(match[3]),
			Severity: "error",
		})
	}
	return errors
}

func parseTscErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := tscPattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		columnNo, _ := strconv.Atoi(match[3])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Column:   columnNo,
			Message:  strings.TrimSpace(match[4]),
			Severity: "error",
		})
	}
	return errors
}

func parseEslintErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, rawLine := range strings.Split(stderr, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "/") || strings.HasPrefix(line, "✖") {
			continue
		}
		match := genericErrorPattern.FindStringSubmatch(line)
		if len(match) == 5 {
			lineNo, _ := strconv.Atoi(match[2])
			columnNo := 0
			if match[3] != "" {
				columnNo, _ = strconv.Atoi(match[3])
			}
			errors = append(errors, ErrorEntry{
				File:     match[1],
				Line:     lineNo,
				Column:   columnNo,
				Message:  strings.TrimSpace(match[4]),
				Severity: "error",
			})
			continue
		}
		if eslintSummaryPattern.MatchString(line) {
			errors = append(errors, ErrorEntry{
				Message:  line,
				Severity: "error",
			})
		}
	}
	return errors
}

func parseGenericErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := genericErrorPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 5 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		columnNo := 0
		if match[3] != "" {
			columnNo, _ = strconv.Atoi(match[3])
		}
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Column:   columnNo,
			Message:  strings.TrimSpace(match[4]),
			Severity: "error",
		})
	}
	return errors
}

func collectResultErrors(result Result) []ErrorEntry {
	if len(result.Errors) > 0 {
		return append([]ErrorEntry(nil), result.Errors...)
	}
	errors := collectStepErrors(result.Steps)
	if len(errors) == 0 {
		return nil
	}
	return errors
}

func collectStepErrors(steps []StepResult) []ErrorEntry {
	var errors []ErrorEntry
	for _, step := range steps {
		if len(step.Errors) > 0 {
			errors = append(errors, step.Errors...)
			continue
		}
		errors = append(errors, collectStepErrors(step.Steps)...)
	}
	return errors
}
