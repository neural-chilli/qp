package runner

import "path/filepath"

// .qp/ subdirectory paths.
// All paths under the .qp/ project directory must be defined here so they can
// be managed centrally. When P2 adds new storage (e.g. SQLite state), add the
// path constants here.

const (
	// DotQPDir is the top-level project state directory.
	DotQPDir = ".qp"

	// CacheSubdir is the subdirectory for task result caches.
	CacheSubdir = "cache"

	// LastGuardFile is the filename for the last guard run result.
	LastGuardFile = "last-guard.json"
)

// CacheDir returns the absolute path to the cache directory for a given repo root.
func CacheDir(repoRoot string) string {
	return filepath.Join(repoRoot, DotQPDir, CacheSubdir)
}

// LastGuardPath returns the absolute path to the last-guard.json file.
func LastGuardPath(repoRoot string) string {
	return filepath.Join(repoRoot, DotQPDir, LastGuardFile)
}
