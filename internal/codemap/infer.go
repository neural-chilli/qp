package codemap

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

type inferrer struct {
	name       string
	extensions map[string]bool
	infer      func(relDir string, files []string) *config.CodemapPackage
}

var (
	goTypeRE       = regexp.MustCompile(`^type\s+([A-Z][A-Za-z0-9_]*)\s+(struct|interface|enum)`)
	goFuncRE       = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?([A-Z][A-Za-z0-9_]*)\s*\(`)
	pyClassRE      = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	pyFuncRE       = regexp.MustCompile(`^def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	jsTypeRE       = regexp.MustCompile(`^export\s+(?:default\s+)?(?:class|interface|type)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	jsFuncRE       = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)?`)
	rustTypeRE     = regexp.MustCompile(`^pub\s+(?:struct|enum|trait)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustFuncRE     = regexp.MustCompile(`^pub\s+(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	javaTypeRE     = regexp.MustCompile(`^public\s+(?:class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	javaStaticRE   = regexp.MustCompile(`^public\s+static\s+[A-Za-z0-9_<>\[\],\s]+\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	cppTypeRE      = regexp.MustCompile(`^(?:class|struct)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	cppFunctionRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_\s\*]*\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;]*\)\s*;`)
	skipDirNames   = map[string]bool{".git": true, ".qp": true, "node_modules": true, "vendor": true, "dist": true, "build": true}
	maxTypes       = 4
	maxEntryPoints = 6
)

var languageInferrers = []inferrer{
	{
		name: "go",
		extensions: map[string]bool{
			".go": true,
		},
		infer: inferGoPackage,
	},
	{
		name: "python",
		extensions: map[string]bool{
			".py": true,
		},
		infer: inferPythonPackage,
	},
	{
		name: "js_ts",
		extensions: map[string]bool{
			".ts":  true,
			".tsx": true,
			".js":  true,
			".jsx": true,
		},
		infer: inferJSPackage,
	},
	{
		name: "rust",
		extensions: map[string]bool{
			".rs": true,
		},
		infer: inferRustPackage,
	},
	{
		name: "java",
		extensions: map[string]bool{
			".java": true,
		},
		infer: inferJavaPackage,
	},
	{
		name: "c_cpp",
		extensions: map[string]bool{
			".h":   true,
			".hpp": true,
			".c":   true,
			".cc":  true,
			".cpp": true,
		},
		infer: inferCPPPackage,
	},
}

func Infer(repoRoot string) (map[string]config.CodemapPackage, error) {
	filesByDir := map[string][]string{}
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDirNames[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}
		if !isSupportedExtension(ext) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." {
			return nil
		}
		filesByDir[dir] = append(filesByDir[dir], path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := map[string]config.CodemapPackage{}
	dirs := make([]string, 0, len(filesByDir))
	for dir := range filesByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		files := filesByDir[dir]
		sort.Strings(files)
		inferrer := pickInferrer(files)
		if inferrer == nil {
			continue
		}
		pkg := inferrer.infer(dir, files)
		if pkg == nil || strings.TrimSpace(pkg.Desc) == "" {
			continue
		}
		out[dir] = *pkg
	}
	return out, nil
}

func isSupportedExtension(ext string) bool {
	for _, inf := range languageInferrers {
		if inf.extensions[ext] {
			return true
		}
	}
	return false
}

func pickInferrer(files []string) *inferrer {
	bestScore := 0
	var best *inferrer
	for i := range languageInferrers {
		score := 0
		for _, file := range files {
			if languageInferrers[i].extensions[strings.ToLower(filepath.Ext(file))] {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = &languageInferrers[i]
		}
	}
	return best
}

func inferGoPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findFirstGoPackageDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s package", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, goTypeRE, maxTypes, true),
		EntryPoints: collectMatches(files, goFuncRE, maxEntryPoints, true),
	}
}

func inferPythonPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findPythonDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s module", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, pyClassRE, maxTypes, false),
		EntryPoints: collectMatches(files, pyFuncRE, maxEntryPoints, false),
	}
}

func inferJSPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findFirstBlockDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s package", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, jsTypeRE, maxTypes, false),
		EntryPoints: collectJSFunctions(files, maxEntryPoints),
	}
}

func inferRustPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findRustDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s crate module", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, rustTypeRE, maxTypes, false),
		EntryPoints: collectMatches(files, rustFuncRE, maxEntryPoints, false),
	}
}

func inferJavaPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findFirstBlockDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s package", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, javaTypeRE, maxTypes, false),
		EntryPoints: collectMatches(files, javaStaticRE, maxEntryPoints, false),
	}
}

func inferCPPPackage(relDir string, files []string) *config.CodemapPackage {
	desc := findFirstBlockDoc(files)
	if desc == "" {
		desc = fmt.Sprintf("%s module", filepath.Base(relDir))
	}
	return &config.CodemapPackage{
		Desc:        desc,
		KeyTypes:    collectMatches(files, cppTypeRE, maxTypes, false),
		EntryPoints: collectMatches(files, cppFunctionRE, maxEntryPoints, false),
	}
}

func collectMatches(files []string, re *regexp.Regexp, limit int, exportedOnly bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		for _, line := range readLines(file, 600) {
			matches := re.FindStringSubmatch(strings.TrimSpace(line))
			if len(matches) < 2 {
				continue
			}
			name := strings.TrimSpace(matches[1])
			if name == "" || seen[name] {
				continue
			}
			if exportedOnly && !isExported(name) {
				continue
			}
			seen[name] = true
			out = append(out, name)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func collectJSFunctions(files []string, limit int) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		for _, line := range readLines(file, 600) {
			matches := jsFuncRE.FindStringSubmatch(strings.TrimSpace(line))
			if len(matches) < 2 {
				continue
			}
			name := strings.TrimSpace(matches[1])
			if name == "" {
				name = "default"
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func findFirstGoPackageDoc(files []string) string {
	for _, file := range files {
		if filepath.Ext(file) != ".go" {
			continue
		}
		for _, line := range readLines(file, 80) {
			text := strings.TrimSpace(line)
			if strings.HasPrefix(text, "// Package ") {
				return strings.TrimPrefix(text, "// Package ")
			}
		}
	}
	return ""
}

func findPythonDoc(files []string) string {
	preferred := ""
	for _, file := range files {
		if strings.HasSuffix(file, "__init__.py") {
			preferred = file
			break
		}
	}
	if preferred != "" {
		if doc := findTripleQuoteDoc(preferred); doc != "" {
			return doc
		}
	}
	for _, file := range files {
		if doc := findTripleQuoteDoc(file); doc != "" {
			return doc
		}
	}
	return ""
}

func findRustDoc(files []string) string {
	for _, file := range files {
		base := filepath.Base(file)
		if base != "lib.rs" && base != "mod.rs" {
			continue
		}
		for _, line := range readLines(file, 80) {
			text := strings.TrimSpace(line)
			if strings.HasPrefix(text, "//!") {
				return strings.TrimSpace(strings.TrimPrefix(text, "//!"))
			}
		}
	}
	return ""
}

func findFirstBlockDoc(files []string) string {
	for _, file := range files {
		lines := readLines(file, 120)
		var block []string
		inBlock := false
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if !inBlock {
				if strings.HasPrefix(line, "/**") || strings.HasPrefix(line, "/*") {
					inBlock = true
					line = strings.TrimPrefix(line, "/**")
					line = strings.TrimPrefix(line, "/*")
				} else {
					continue
				}
			}
			line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
			line = strings.TrimSuffix(line, "*/")
			line = strings.TrimSpace(line)
			if line != "" {
				block = append(block, line)
			}
			if strings.Contains(raw, "*/") {
				if len(block) > 0 {
					return block[0]
				}
				break
			}
		}
	}
	return ""
}

func findTripleQuoteDoc(file string) string {
	lines := readLines(file, 120)
	inDoc := false
	var doc []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if !inDoc {
			if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, `'''`) {
				inDoc = true
				line = strings.TrimPrefix(line, `"""`)
				line = strings.TrimPrefix(line, `'''`)
			} else {
				continue
			}
		}
		line = strings.TrimSuffix(line, `"""`)
		line = strings.TrimSuffix(line, `'''`)
		line = strings.TrimSpace(line)
		if line != "" {
			doc = append(doc, line)
		}
		if strings.Contains(raw, `"""`) || strings.Contains(raw, `'''`) {
			if len(doc) > 0 {
				return doc[0]
			}
			break
		}
	}
	return ""
}

func readLines(path string, max int) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	lines := make([]string, 0, max)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= max {
			break
		}
	}
	return lines
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}
