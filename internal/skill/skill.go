// Package skill serves the guidance shipped in the executable.
package skill

import (
	"embed"
	"slices"
)

//go:embed core.md topics/*.md
var documents embed.FS

var names = [...]string{"import", "claude-artifacts", "google-docs", "browser-use", "semantic", "schema"}

// Core returns the exact embedded Markdown, including its final LF.
func Core() string { return embeddedText("core.md") }

// Topics returns the public names in display order.
func Topics() []string { return slices.Clone(names[:]) }

// Topic returns the exact embedded text of a named topic.
func Topic(name string) (text string, ok bool) {
	for _, topic := range names {
		if topic == name {
			return embeddedText("topics/" + topic + ".md"), true
		}
	}
	return "", false
}

func embeddedText(path string) string {
	data, err := documents.ReadFile(path)
	if err != nil {
		panic("skill: missing embedded document " + path)
	}
	return string(data)
}
