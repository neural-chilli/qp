package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/ordered"
)

func writeDocs(repoRoot string, cfg *config.Config) ([]string, error) {
	statuses := make([]string, 0, 3)
	items := []struct {
		path  string
		start string
		end   string
		body  string
	}{
		{path: filepath.Join(repoRoot, "HUMANS.md"), start: humansBlockStart, end: humansBlockEnd, body: renderHumansDoc(cfg)},
		{path: filepath.Join(repoRoot, "AGENTS.md"), start: agentsBlockStart, end: agentsBlockEnd, body: renderAgentDoc("AGENTS", cfg)},
		{path: filepath.Join(repoRoot, "CLAUDE.md"), start: claudeBlockStart, end: claudeBlockEnd, body: renderAgentDoc("CLAUDE", cfg)},
	}
	for _, item := range items {
		status, err := writeManagedDoc(item.path, item.start, item.end, item.body)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func renderHumansDoc(cfg *config.Config) string {
	var builder strings.Builder
	builder.WriteString("# HUMANS\n\n")
	builder.WriteString("This file is generated from `qp.yaml` and summarizes how this repository is intended to be used by people.\n\n")
	builder.WriteString("## Project\n\n")
	builder.WriteString(fmt.Sprintf("- Project: `%s`\n", cfg.Project))
	if cfg.Description != "" {
		builder.WriteString(fmt.Sprintf("- Description: %s\n", cfg.Description))
	}
	if cfg.Default != "" {
		builder.WriteString(fmt.Sprintf("- Default command: `qp %s`\n", cfg.Default))
	}
	builder.WriteString("\n## Recommended Start\n\n")
	builder.WriteString("1. `qp list`\n")
	builder.WriteString("2. `qp help <task>`\n")
	builder.WriteString("3. `qp context`\n")
	builder.WriteString("4. `qp guard` or `qp <task>`\n")
	writeTaskSummary(&builder, cfg, false)
	writeGuardSection(&builder, cfg)
	writeGroupSection(&builder, cfg)
	writeScopeSection(&builder, cfg)
	writePromptSection(&builder, cfg)
	writeCodemapSection(&builder, cfg)
	return builder.String()
}

func renderAgentDoc(name string, cfg *config.Config) string {
	var builder strings.Builder
	goldenCommand := "qp guard"
	if len(cfg.Guards) == 0 {
		if cfg.Default != "" {
			goldenCommand = "qp " + cfg.Default
		} else {
			goldenCommand = "qp <task>"
		}
	}
	builder.WriteString("# " + name + "\n\n")
	builder.WriteString("This file is generated from `qp.yaml` and is the repo-specific workflow guide for coding agents.\n\n")
	builder.WriteString("## One Command\n\n")
	builder.WriteString(fmt.Sprintf("Run `%s` after making changes to verify the repo state before reporting completion.\n\n", goldenCommand))
	builder.WriteString("## Agent Conventions\n\n")
	builder.WriteString(fmt.Sprintf("- Always run `%s` before reporting work as complete.\n", goldenCommand))
	builder.WriteString("- Always prefer an existing `qp` task over ad hoc shell commands.\n")
	builder.WriteString("- Never run `destructive` or `external` tasks unless explicitly asked.\n")
	builder.WriteString("- Never edit files outside the relevant task scope unless the user asks.\n")
	builder.WriteString("## How To Work Here\n\n")
	builder.WriteString("- Start with `qp list` to discover the available tasks.\n")
	builder.WriteString("- Use `qp help <task>` before inventing an equivalent command.\n")
	builder.WriteString("- Use `qp context` for a bounded repo summary.\n")
	builder.WriteString("- Use `qp agent-brief` when you want one combined handoff artifact.\n")
	builder.WriteString("- Use `qp guard` when you want a validation report across multiple checks.\n")
	builder.WriteString("- Prefer `qp` tasks over ad hoc shell commands when the task already exists.\n")
	if cfg.Agent.AccrueKnowledge {
		builder.WriteString("- When you discover operational knowledge, propose a structured update to `qp.yaml`, then run `qp validate` before suggesting it.\n")
	}
	builder.WriteString("\n## Project\n\n")
	builder.WriteString(fmt.Sprintf("- Project: `%s`\n", cfg.Project))
	if cfg.Description != "" {
		builder.WriteString(fmt.Sprintf("- Description: %s\n", cfg.Description))
	}
	if cfg.Default != "" {
		builder.WriteString(fmt.Sprintf("- Default command: `qp %s`\n", cfg.Default))
	}
	writeTaskSummary(&builder, cfg, true)
	writeGuardSection(&builder, cfg)
	writeGroupSection(&builder, cfg)
	writeScopeSection(&builder, cfg)
	writePromptSection(&builder, cfg)
	writeCodemapSection(&builder, cfg)
	writeContextSection(&builder, cfg)
	writeWatchSection(&builder, cfg)
	if cfg.Agent.AccrueKnowledge {
		writeKnowledgeSection(&builder)
	}
	return builder.String()
}

func writeTaskSummary(builder *strings.Builder, cfg *config.Config, includeAgentFields bool) {
	builder.WriteString("\n## Tasks\n\n")
	for _, name := range ordered.Keys(cfg.Tasks) {
		task := cfg.Tasks[name]
		builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, task.Desc))
		if task.Scope != "" {
			builder.WriteString(fmt.Sprintf("  Scope: `%s`\n", task.Scope))
			if scopeDef, ok := cfg.Scopes[task.Scope]; ok && scopeDef.Desc != "" {
				builder.WriteString(fmt.Sprintf("  Scope Description: %s\n", scopeDef.Desc))
			}
		}
		if len(task.Steps) > 0 {
			builder.WriteString(fmt.Sprintf("  Steps: `%s`\n", strings.Join(task.Steps, "`, `")))
		}
		if len(task.Needs) > 0 {
			builder.WriteString(fmt.Sprintf("  Needs: `%s`\n", strings.Join(task.Needs, "`, `")))
		}
		if task.Cmd != "" {
			builder.WriteString(fmt.Sprintf("  Command: `%s`\n", task.Cmd))
		}
		if includeAgentFields {
			builder.WriteString(fmt.Sprintf("  Agent visible: `%t`\n", task.AgentEnabled()))
			builder.WriteString(fmt.Sprintf("  Safety: `%s`\n", task.SafetyLevel()))
		}
	}
}

func writeGuardSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Guards) == 0 {
		return
	}
	builder.WriteString("\n## Guards\n\n")
	for _, name := range ordered.Keys(cfg.Guards) {
		guardCfg := cfg.Guards[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(guardCfg.Steps, "`, `")))
	}
}

func writeGroupSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Groups) == 0 {
		return
	}
	builder.WriteString("\n## Groups\n\n")
	for _, name := range ordered.Keys(cfg.Groups) {
		group := cfg.Groups[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(group.Tasks, "`, `")))
		if group.Desc != "" {
			builder.WriteString(fmt.Sprintf("  Description: %s\n", group.Desc))
		}
	}
}

func writeScopeSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Scopes) == 0 {
		return
	}
	builder.WriteString("\n## Scopes\n\n")
	for _, name := range ordered.Keys(cfg.Scopes) {
		scopeDef := cfg.Scopes[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(scopeDef.Paths, "`, `")))
		if scopeDef.Desc != "" {
			builder.WriteString(fmt.Sprintf("  Description: %s\n", scopeDef.Desc))
		}
	}
}

func writePromptSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Prompts) == 0 {
		return
	}
	builder.WriteString("\n## Prompts\n\n")
	for _, name := range ordered.Keys(cfg.Prompts) {
		promptCfg := cfg.Prompts[name]
		builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, promptCfg.Desc))
	}
}

func writeCodemapSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Codemap.Packages) == 0 && len(cfg.Codemap.Conventions) == 0 && len(cfg.Codemap.Glossary) == 0 {
		return
	}
	builder.WriteString("\n## Codemap\n\n")
	if len(cfg.Codemap.Packages) > 0 {
		for _, name := range ordered.Keys(cfg.Codemap.Packages) {
			pkg := cfg.Codemap.Packages[name]
			builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, pkg.Desc))
		}
	}
	if len(cfg.Codemap.Conventions) > 0 {
		builder.WriteString("\nConventions:\n")
		for _, convention := range cfg.Codemap.Conventions {
			builder.WriteString(fmt.Sprintf("- %s\n", convention))
		}
	}
	if len(cfg.Codemap.Glossary) > 0 {
		builder.WriteString("\nGlossary:\n")
		for _, term := range ordered.Keys(cfg.Codemap.Glossary) {
			builder.WriteString(fmt.Sprintf("- `%s`: %s\n", term, cfg.Codemap.Glossary[term]))
		}
	}
}

func writeContextSection(builder *strings.Builder, cfg *config.Config) {
	builder.WriteString("\n## Context\n\n")
	builder.WriteString("- Use `qp context` for a general repo briefing.\n")
	builder.WriteString("- Use `qp context --agent --task <task>` when working on a specific task.\n")
	if len(cfg.Context.AgentFiles) > 0 {
		builder.WriteString(fmt.Sprintf("- Agent files: `%s`\n", strings.Join(cfg.Context.AgentFiles, "`, `")))
	}
	if len(cfg.Context.Include) > 0 {
		builder.WriteString(fmt.Sprintf("- Included paths: `%s`\n", strings.Join(cfg.Context.Include, "`, `")))
	}
}

func writeWatchSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Watch.Paths) == 0 {
		return
	}
	builder.WriteString("\n## Watch\n\n")
	builder.WriteString(fmt.Sprintf("- Watched paths: `%s`\n", strings.Join(cfg.Watch.Paths, "`, `")))
	builder.WriteString(fmt.Sprintf("- Debounce: `%dms`\n", cfg.Watch.DebounceMS))
}

func writeKnowledgeSection(builder *strings.Builder) {
	builder.WriteString("\n## Knowledge Accrual\n\n")
	builder.WriteString("When you discover operational knowledge about this repository, propose a structured update to `qp.yaml`.\n\n")
	builder.WriteString("- Build prerequisites or ordering -> task `needs`\n")
	builder.WriteString("- Required environment -> task `env` or `params`\n")
	builder.WriteString("- Files that should not be edited -> scope paths or context `exclude`\n")
	builder.WriteString("- Package purpose or structure -> `codemap.packages`\n")
	builder.WriteString("- Naming conventions -> `codemap.conventions`\n")
	builder.WriteString("- Domain terminology -> `codemap.glossary`\n")
	builder.WriteString("- Run `qp validate` before proposing the change.\n")
	builder.WriteString("- Always confirm with the user before applying it.\n")
}

func writeManagedDoc(path, start, end, body string) (string, error) {
	block := strings.Join([]string{start, body, end, ""}, "\n")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	content := string(raw)
	statusPrefix := "wrote "
	if strings.Contains(content, start) && strings.Contains(content, end) {
		startIdx := strings.Index(content, start)
		endIdx := strings.Index(content, end)
		if startIdx >= 0 && endIdx >= startIdx {
			endIdx += len(end)
			content = content[:startIdx] + block + strings.TrimLeft(content[endIdx:], "\n")
			statusPrefix = "updated "
		}
	} else if strings.TrimSpace(content) == "" {
		content = block
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + block
		statusPrefix = "updated "
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return statusPrefix + filepath.Base(path), nil
}

func ensureGitignoreEntry(path, entry string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	if len(raw) > 0 {
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				return false, nil
			}
		}
	}

	content := strings.TrimRight(string(raw), "\n")
	if content == "" {
		content = entry + "\n"
	} else {
		content = content + "\n" + entry + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
