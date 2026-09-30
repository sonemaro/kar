package main

import "strings"

// textToADF converts plain text into a minimal Atlassian Document Format
// document: double newlines separate paragraphs, single newlines become
// spaces (Jira paragraph text has no soft breaks). Enough for comments and
// short descriptions; rich formatting is out of scope for a sync utility.
func textToADF(text string) map[string]any {
	paras := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	content := make([]any, 0, len(paras))
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.Join(strings.Fields(p), " ")
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []any{map[string]any{
				"type": "text",
				"text": p,
			}},
		})
	}
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": []any{},
		})
	}
	return map[string]any{"type": "doc", "version": 1, "content": content}
}
