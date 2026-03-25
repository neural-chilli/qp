package initcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/qp/internal/codemap"
	"github.com/neural-chilli/qp/internal/config"
)

type Options struct {
	FromRepo bool
	Docs     bool
	Harness  bool
	Codemap  bool
}

const starterConfigBase = `project: my-project
description: Describe your repository
default: check

tasks:
  test:
    desc: Run the test suite
    scope: cli
    cmd: go test ./...

  build:
    desc: Build the application
    scope: cli
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    scope: cli
    steps:
      - test
      - build

groups:
  core:
    desc: Everyday local development commands.
    tasks:
      - test
      - build
      - check

scopes:
  cli:
    desc: Main CLI commands and closely-related execution packages.
    paths:
      - cmd/
      - internal/

agent:
  accrue_knowledge: false
`

const starterConfigHarness = `
architecture:
  layers:
    - types
    - config
    - repo
    - service
    - runtime
    - ui
  domains:
    app:
      root: internal
      layers:
        - types
        - config
        - repo
        - service
        - runtime
        - ui
  rules:
    - direction: forward
    - cross_domain: deny
`

func starterConfig(harness bool) string {
	if !harness {
		return starterConfigBase
	}
	return `project: my-project
description: Describe your repository
default: check

tasks:
  test:
    desc: Run the test suite
    scope: cli
    cmd: go test ./...

  build:
    desc: Build the application
    scope: cli
    cmd: go build ./...

  arch-check:
    desc: Validate architecture boundaries
    scope: cli
    cmd: qp arch-check

  check:
    desc: Run the default local verification pipeline
    scope: cli
    steps:
      - test
      - build
      - arch-check

groups:
  core:
    desc: Everyday local development commands.
    tasks:
      - test
      - build
      - arch-check
      - check

scopes:
  cli:
    desc: Main CLI commands and closely-related execution packages.
    paths:
      - cmd/
      - internal/

` + strings.TrimPrefix(starterConfigHarness, "\n") + `
agent:
  accrue_knowledge: false
`
}

const (
	humansBlockStart = "<!-- qp:humans:start -->"
	humansBlockEnd   = "<!-- qp:humans:end -->"
	agentsBlockStart = "<!-- qp:agents:start -->"
	agentsBlockEnd   = "<!-- qp:agents:end -->"
	claudeBlockStart = "<!-- qp:claude:start -->"
	claudeBlockEnd   = "<!-- qp:claude:end -->"
)

type inferredTask struct {
	Name   string
	Desc   string
	Cmd    string
	Steps  []string
	Agent  *bool
	Safety string
	Params map[string]config.Param
}

type makeTarget struct {
	Name   string
	Params []string
}

type justRecipe struct {
	Name   string
	Params []justParam
}

type justParam struct {
	Name     string
	Default  string
	Required bool
	Variadic bool
}

type packageScript struct {
	Name   string
	Cmd    string
	Params map[string]config.Param
}

func Run(repoRoot string, opts Options) (string, error) {
	var messages []string

	cfgPath := filepath.Join(repoRoot, "qp.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		messages = append(messages, "qp.yaml already exists; leaving it unchanged")
	} else if os.IsNotExist(err) {
		configBody := starterConfig(opts.Harness)
		if opts.FromRepo {
			configBody = inferConfig(repoRoot)
		}
		if err := os.WriteFile(cfgPath, []byte(configBody), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "created qp.yaml")
		if opts.FromRepo {
			messages = append(messages, "scaffolded tasks from existing repo files")
			if !opts.Docs {
				messages = append(messages, "tip: run `qp init --docs` to generate HUMANS.md, AGENTS.md, and CLAUDE.md")
			}
		} else if opts.Harness {
			messages = append(messages, "added harness architecture scaffold")
		}
	} else {
		return "", err
	}

	updated, err := ensureGitignoreEntry(filepath.Join(repoRoot, ".gitignore"), ".qp/")
	if err != nil {
		return "", err
	}
	if updated {
		messages = append(messages, "updated .gitignore with .qp/")
	} else {
		messages = append(messages, ".gitignore already includes .qp/")
	}

	if opts.Docs {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return "", err
		}
		statuses, err := writeDocs(repoRoot, cfg)
		if err != nil {
			return "", err
		}
		messages = append(messages, statuses...)
	}
	if opts.Codemap {
		inferred, err := codemap.Infer(repoRoot)
		if err != nil {
			return "", err
		}
		status, err := mergeInferredCodemap(cfgPath, inferred)
		if err != nil {
			return "", err
		}
		messages = append(messages, status)
	}

	return strings.Join(messages, "\n"), nil
}
