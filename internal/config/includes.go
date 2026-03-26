package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// IncludeEntry represents a single include directive — either a plain file/glob
// path or a namespaced directory.
type IncludeEntry struct {
	Path      string // file path, glob pattern, or directory
	Namespace string // non-empty for namespaced includes (e.g. "ml")
}

// IncludeList is a custom YAML type that accepts both list and map forms:
//
//	# list form (plain paths and globs)
//	includes:
//	  - tasks/backend.yaml
//	  - tasks/**/*.yaml
//
//	# map form (namespaced directories)
//	includes:
//	  ml: tasks/ml/
//	  data: tasks/data/
type IncludeList []IncludeEntry

func (il *IncludeList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("includes list items must be strings")
			}
			*il = append(*il, IncludeEntry{Path: item.Value})
		}
		return nil
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			key := value.Content[i]
			val := value.Content[i+1]
			if key.Kind != yaml.ScalarNode || val.Kind != yaml.ScalarNode {
				return fmt.Errorf("includes map entries must be string: string")
			}
			*il = append(*il, IncludeEntry{Path: val.Value, Namespace: key.Value})
		}
		return nil
	default:
		return fmt.Errorf("includes must be a list or map")
	}
}

// resolveIncludeFiles expands a path into concrete file paths. Handles:
//   - plain files: returns as-is
//   - directories: returns all *.yaml and *.yml files within (recursive)
//   - glob patterns: expands, including ** for recursive matching
func resolveIncludeFiles(baseDir, pattern string) ([]string, error) {
	target := pattern
	if !filepath.IsAbs(target) {
		target = filepath.Join(baseDir, target)
	}

	// Check if it's a directory.
	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		return yamlFilesInDir(target)
	}

	// Try as glob (supports ** via walkGlob).
	if strings.Contains(pattern, "*") {
		matches, err := walkGlob(baseDir, pattern)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files match pattern %q", pattern)
		}
		sort.Strings(matches)
		return matches, nil
	}

	// Plain file.
	if _, err := os.Stat(target); err != nil {
		return nil, err
	}
	return []string{target}, nil
}

// yamlFilesInDir returns all .yaml and .yml files under dir, recursively.
func yamlFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// walkGlob expands glob patterns including ** for recursive directory matching.
func walkGlob(baseDir, pattern string) ([]string, error) {
	abs := pattern
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(baseDir, abs)
	}

	// If the pattern contains **, walk the filesystem and match.
	if strings.Contains(abs, "**") {
		return walkDoubleStarGlob(abs)
	}

	// Single-star glob — use stdlib.
	matches, err := filepath.Glob(abs)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// walkDoubleStarGlob handles patterns like dir/**/file.yaml by walking the
// root prefix and matching the suffix against each candidate path.
func walkDoubleStarGlob(pattern string) ([]string, error) {
	// Split at the first ** to get the root directory to walk.
	idx := strings.Index(pattern, "**")
	root := filepath.Clean(pattern[:idx])

	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if doubleStarMatch(pattern, path) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// doubleStarMatch tests whether path matches a glob pattern containing **.
// ** matches zero or more directory segments.
func doubleStarMatch(pattern, path string) bool {
	patParts := splitPath(pattern)
	pathParts := splitPath(path)
	return matchParts(patParts, pathParts)
}

func splitPath(p string) []string {
	p = filepath.Clean(p)
	return strings.Split(p, string(filepath.Separator))
}

func matchParts(pattern, path []string) bool {
	pi, pa := 0, 0
	for pi < len(pattern) && pa < len(path) {
		if pattern[pi] == "**" {
			// Skip consecutive ** segments.
			for pi < len(pattern) && pattern[pi] == "**" {
				pi++
			}
			if pi == len(pattern) {
				return true // ** at end matches everything
			}
			// Try matching the rest from every position.
			for pa <= len(path) {
				if matchParts(pattern[pi:], path[pa:]) {
					return true
				}
				pa++
			}
			return false
		}
		matched, err := filepath.Match(pattern[pi], path[pa])
		if err != nil || !matched {
			return false
		}
		pi++
		pa++
	}
	return pi == len(pattern) && pa == len(path)
}

// prefixNamespacedTasks rewrites task names and internal references with a
// namespace prefix. References that match a task in localNames get prefixed;
// others are left untouched (they may refer to root-level or cross-namespace tasks).
func prefixNamespacedTasks(tasks map[string]Task, namespace string) map[string]Task {
	localNames := make(map[string]bool, len(tasks))
	for name := range tasks {
		localNames[name] = true
	}

	rename := make(map[string]string, len(tasks))
	for name := range tasks {
		rename[name] = namespace + ":" + name
	}

	prefixed := make(map[string]Task, len(tasks))
	for name, task := range tasks {
		task = rewriteTaskRefs(task, localNames, rename)
		prefixed[namespace+":"+name] = task
	}
	return prefixed
}

// rewriteTaskRefs rewrites steps, needs, and run references in a task.
func rewriteTaskRefs(task Task, localNames map[string]bool, rename map[string]string) Task {
	for i, step := range task.Steps {
		if localNames[step] {
			task.Steps[i] = rename[step]
		}
	}
	for i, need := range task.Needs {
		if localNames[need] {
			task.Needs[i] = rename[need]
		}
	}
	if task.Run != "" {
		expr, err := ParseRunExpr(task.Run)
		if err == nil {
			rewritten := RunExprRenameRefs(expr, rename)
			task.Run = RunExprString(rewritten)
		}
	}
	return task
}
