// Package secretscan: exported surface from the pbrain specs; bodies are filled in by later phases.
package secretscan

type Pattern string // pem_private_key, aws_access_key, github_token, slack_token
type Field struct {
	Name  string
	Value string
}
type Match struct {
	Field   string
	Line    int
	Pattern Pattern
}

func Scan(fields []Field) []Match { panic("not implemented") }
func Sensitive(value string) bool { panic("not implemented") }
