package aggregator

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

var (
	htmlTagRegex  = regexp.MustCompile(`<[^>]*>`)
	spaceRegex    = regexp.MustCompile(`\s+`)
	knownModelMap = []struct {
		Tag   string
		Match []string
	}{
		{"DeepSeek", []string{"deepseek", "deepseek-r1", "deepseek-v3", "deepseek-coder", "mla"}},
		{"Gemini", []string{"gemini", "gemma", "deepmind", "alphafold", "veo"}},
		{"Claude", []string{"claude", "anthropic", "sonnet", "haiku", "opus"}},
		{"GPT / OpenAI", []string{"gpt-4", "gpt-4o", "o1-preview", "o1-mini", "o3-mini", "openai", "chatgpt", "sora"}},
		{"Llama", []string{"llama", "llama-3", "llama 3", "llama-3.3", "meta ai"}},
		{"Qwen", []string{"qwen", "qwen2", "qwen2.5", "qwq"}},
		{"Mistral", []string{"mistral", "codestral", "pixtral", "mixtral"}},
		{"Flux / Image", []string{"flux.1", "black forest labs", "diffusion transformer", "stable diffusion", "midjourney"}},
		{"Phi", []string{"phi-3", "phi-4", "microsoft phi"}},
		{"Grok", []string{"grok", "xai"}},
	}
)

// CleanText strips HTML tags, decodes entities, and normalizes whitespace
func CleanText(raw string) string {
	cleaned := htmlTagRegex.ReplaceAllString(raw, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = spaceRegex.ReplaceAllLiteralString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

// ExtractModelTags identifies AI models referenced in title or text
func ExtractModelTags(title, summary string) []string {
	content := strings.ToLower(title + " " + summary)
	tagSet := make(map[string]bool)

	for _, item := range knownModelMap {
		for _, kw := range item.Match {
			if strings.Contains(content, kw) {
				tagSet[item.Tag] = true
				break
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// Categorize determines content categories based on text markers
func Categorize(title, summary, source string, isPaper bool) []string {
	content := strings.ToLower(title + " " + summary)
	catSet := make(map[string]bool)

	if isPaper || strings.Contains(source, "arXiv") || strings.Contains(source, "Paper") {
		catSet["Research Papers"] = true
	}

	// Model category detectors
	if strings.Contains(content, "reasoning") || strings.Contains(content, "llm") ||
		strings.Contains(content, "language model") || strings.Contains(content, "thinking") ||
		strings.Contains(content, "chain of thought") || strings.Contains(content, "math") ||
		strings.Contains(content, "benchmark") {
		catSet["LLMs & Reasoning"] = true
	}

	if strings.Contains(content, "vision") || strings.Contains(content, "multimodal") ||
		strings.Contains(content, "image") || strings.Contains(content, "video") ||
		strings.Contains(content, "audio") || strings.Contains(content, "diffusion") ||
		strings.Contains(content, "flux") {
		catSet["Multimodal & Vision"] = true
	}

	if strings.Contains(content, "open-source") || strings.Contains(content, "open weights") ||
		strings.Contains(content, "open weight") || strings.Contains(content, "hugging face") ||
		strings.Contains(content, "apache") || strings.Contains(content, "mit license") ||
		strings.Contains(content, "github") {
		catSet["Open Weights"] = true
	}

	if strings.Contains(content, "agent") || strings.Contains(content, "coding") ||
		strings.Contains(content, "swe-bench") || strings.Contains(content, "code generation") ||
		strings.Contains(content, "autonomous") || strings.Contains(content, "tool use") {
		catSet["Agents & Code"] = true
	}

	if strings.Contains(content, "chip") || strings.Contains(content, "nvidia") ||
		strings.Contains(content, "datacenter") || strings.Contains(content, "compute") ||
		strings.Contains(content, "gpu") || strings.Contains(content, "infrastructure") {
		catSet["Infrastructure & Hardware"] = true
	}

	// Default fallback
	if len(catSet) == 0 {
		catSet["AI News"] = true
	}

	cats := make([]string, 0, len(catSet))
	for c := range catSet {
		cats = append(cats, c)
	}
	return cats
}

// FormatTimeAgo converts a timestamp to friendly relative time string
func FormatTimeAgo(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 48*time.Hour {
		return "Yesterday"
	} else {
		days := int(diff.Hours() / 24)
		if days < 30 {
			return fmt.Sprintf("%d days ago", days)
		}
		return t.Format("Jan 02, 2006")
	}
}
