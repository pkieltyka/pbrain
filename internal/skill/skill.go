// Package skill: exported surface from the pbrain specs; bodies are filled in by later phases.
package skill

// Core and Topic return the exact embedded Markdown, including the final LF.
func Core() string                             { panic("not implemented") }
func Topics() []string                         { panic("not implemented") }
func Topic(name string) (text string, ok bool) { panic("not implemented") }
