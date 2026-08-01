package collector

import (
	"strings"
)

// NormalizeModelForGrouping normalizes a model ID for aggregation.
func NormalizeModelForGrouping(modelID string) string {
	name := strings.TrimSpace(strings.ToLower(modelID))
	if name == "" {
		return "unknown"
	}

	// Strip reasoning tier suffix: "claude-sonnet-4-20250514 (high)" -> "claude-sonnet-4-20250514"
	reasoningTiers := map[string]bool{
		"minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "auto": true, "none": true,
	}
	if strings.HasSuffix(name, ")") {
		openIdx := strings.LastIndex(name, "(")
		if openIdx > 0 {
			tier := strings.TrimSpace(name[openIdx+1 : len(name)-1])
			if reasoningTiers[tier] {
				name = strings.TrimSpace(name[:openIdx])
			}
		}
	}

	// Strip trailing date suffix like "-20250514"
	if len(name) > 9 {
		suffix := name[len(name)-8:]
		if isDigits8(suffix) && name[len(name)-9] == '-' {
			name = name[:len(name)-9]
		}
	}

	// Claude models: normalize dots to hyphens between digits
	if strings.Contains(name, "claude") {
		name = strings.ReplaceAll(name, ".", "-")
	}

	return name
}

func isDigits8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CanonicalProvider normalizes a raw provider string.
func CanonicalProvider(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(raw), "-", "_"), "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "unknown" {
			continue
		}
		switch part {
		case "x_ai", "xai":
			return "xai"
		case "z_ai", "zai":
			return "zai"
		case "moonshot", "moonshotai":
			return "moonshotai"
		case "meta", "meta_llama":
			return "meta_llama"
		case "azure", "azure_ai":
			return "azure_ai"
		case "anthropic", "vertex", "vertex_ai":
			return "anthropic"
		case "together", "together_ai":
			return "together_ai"
		case "fireworks", "fireworks_ai":
			return "fireworks_ai"
		case "google", "gemini":
			return "google"
		case "openai", "openai_codex":
			return "openai"
		case "mistral", "mistralai":
			return "mistralai"
		case "deepseek":
			return "deepseek"
		case "qwen":
			return "qwen"
		}
		if !containsDigit(part) {
			return part
		}
	}
	return ""
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// InferProviderFromModel tries to guess the provider from a model ID string.
func InferProviderFromModel(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"), strings.Contains(lower, "anthropic"):
		return "anthropic"
	case strings.Contains(lower, "gpt"), strings.Contains(lower, "openai"):
		return "openai"
	case strings.Contains(lower, "gemini"), strings.Contains(lower, "google"):
		return "google"
	case strings.Contains(lower, "grok"):
		return "xai"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "mimo"), strings.Contains(lower, "xiaomi"):
		return "xiaomi"
	case strings.Contains(lower, "mistral"), strings.Contains(lower, "mixtral"):
		return "mistral"
	case strings.Contains(lower, "llama"):
		return "meta_llama"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "glm"):
		return "zai"
	}
	return ""
}
