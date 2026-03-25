package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

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

func writeCachedResult(repoRoot, key string, result Result) {
	dir := filepath.Join(repoRoot, ".qp", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, key+".json"), raw, 0o644)
}
