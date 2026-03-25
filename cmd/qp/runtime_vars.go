package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

func parseVarAssignments(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for _, value := range values {
		name, resolved, ok := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("--var requires name=value")
		}
		out[name] = resolved
	}
	return out, nil
}

func envVarOverridesFromEnviron(environ []string) map[string]string {
	const prefix = "QP_VAR_"
	out := map[string]string{}
	for _, item := range environ {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		name := strings.TrimPrefix(key, prefix)
		if name == "" {
			continue
		}
		out[name] = value
	}
	return out
}

func applyVarOverrides(cfg *config.Config, overrides map[string]string) {
	if len(overrides) == 0 {
		return
	}
	if cfg.Vars == nil {
		cfg.Vars = config.Vars{}
	}
	for name, value := range overrides {
		target := resolveVarName(cfg.Vars, name)
		cfg.Vars[target] = value
	}
}

func resolveVarName(vars config.Vars, name string) string {
	if _, ok := vars[name]; ok {
		return name
	}
	for existing := range vars {
		if strings.EqualFold(existing, name) {
			return existing
		}
	}
	return strings.ToLower(name)
}

func splitProfileEnvValue(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func envProfiles() []string {
	return splitProfileEnvValue(os.Getenv("QP_PROFILE"))
}
