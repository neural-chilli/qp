package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

type cacheKeyInput struct {
	TaskName    string
	Task        config.Task
	ResolvedCmd string
	Params      map[string]string
	Env         map[string]string
	ExtraEnv    map[string]string
	WorkDir     string
	Profile     string
	ContentHash string
}

func makeCacheKey(in cacheKeyInput) string {
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func readCachedResult(repoRoot, key string) (Result, bool) {
	path := filepath.Join(repoRoot, ".qp", "cache", key+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, false
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return Result{}, false
	}
	return result, true
}

func writeCachedResult(repoRoot, key string, result Result) error {
	dir := filepath.Join(repoRoot, ".qp", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal cache result: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), raw, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}
	return nil
}

func hashCachePaths(repoRoot string, patterns []string) (string, error) {
	normalizedPatterns := normalizeCachePatterns(repoRoot, patterns)
	matched := map[string]struct{}{}
	err := filepath.WalkDir(repoRoot, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".qp":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, pattern := range normalizedPatterns {
			if matchGlobPath(pattern, rel) {
				matched[p] = struct{}{}
				break
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	files := make([]string, 0, len(matched))
	for file := range matched {
		files = append(files, file)
	}
	sort.Strings(files)

	hasher := sha256.New()
	for _, file := range files {
		rel, err := filepath.Rel(repoRoot, file)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hasher, filepath.ToSlash(rel))
		_, _ = io.WriteString(hasher, "\n")
		fh, err := os.Open(file)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hasher, fh)
		closeErr := fh.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = io.WriteString(hasher, "\n")
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func normalizeCachePatterns(repoRoot string, patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		p := filepath.ToSlash(strings.TrimSpace(pattern))
		p = strings.TrimPrefix(p, "./")
		if filepath.IsAbs(pattern) {
			rel, err := filepath.Rel(repoRoot, pattern)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			p = filepath.ToSlash(rel)
		}
		p = path.Clean(p)
		if p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}

func matchGlobPath(pattern, relPath string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(relPath, "/")
	return matchGlobParts(patternParts, pathParts)
}

func matchGlobParts(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	if patternParts[0] == "**" {
		if matchGlobParts(patternParts[1:], pathParts) {
			return true
		}
		if len(pathParts) == 0 {
			return false
		}
		return matchGlobParts(patternParts, pathParts[1:])
	}
	if len(pathParts) == 0 {
		return false
	}
	ok, err := path.Match(patternParts[0], pathParts[0])
	if err != nil || !ok {
		return false
	}
	return matchGlobParts(patternParts[1:], pathParts[1:])
}
