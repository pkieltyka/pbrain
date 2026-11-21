// Package secretscan reports the field and line of likely secrets.
package secretscan

import (
	"iter"
	"regexp"
	"strings"
)

type Pattern string

const (
	PEMPrivateKey Pattern = "pem_private_key"
	AWSAccessKey  Pattern = "aws_access_key"
	GitHubToken   Pattern = "github_token"
	SlackToken    Pattern = "slack_token"
)

type Field struct {
	Name  string
	Value string
}

type Match struct {
	Field   string
	Line    int
	Pattern Pattern
}

var patterns = [...]struct {
	kind Pattern
	re   *regexp.Regexp
}{
	{AWSAccessKey, regexp.MustCompile(`(?:AKIA|ASIA)[A-Z0-9]{16}`)},
	{GitHubToken, regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{20,255}`)},
	{PEMPrivateKey, regexp.MustCompile(`-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----`)},
	{SlackToken, regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,255}`)},
}

// Scan reports one match per pattern on each field line, without the matched value.
func Scan(fields []Field) []Match {
	var found []Match
	for _, field := range fields {
		for line, pattern := range locations(field.Value) {
			found = append(found, Match{Field: field.Name, Line: line, Pattern: pattern})
		}
	}
	return found
}

// Sensitive reports whether Scan would find any match in value.
func Sensitive(value string) bool {
	for range locations(value) {
		return true
	}
	return false
}

// locations yields field line numbers, then pattern names in lexical order.
func locations(value string) iter.Seq2[int, Pattern] {
	return func(yield func(int, Pattern) bool) {
		line := 0
		for text := range strings.SplitSeq(value, "\n") {
			line++
			for _, pattern := range patterns {
				if matches(text, pattern.kind, pattern.re) && !yield(line, pattern.kind) {
					return
				}
			}
		}
	}
}

func matches(line string, kind Pattern, re *regexp.Regexp) bool {
	if kind == PEMPrivateKey {
		return re.MatchString(line)
	}
	for _, span := range re.FindAllStringIndex(line, -1) {
		start, end := span[0], span[1]
		if start > 0 && joined(line[start-1], kind) || end < len(line) && joined(line[end], kind) {
			continue
		}
		return true
	}
	return false
}

func joined(b byte, kind Pattern) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' ||
		b == '_' || kind == SlackToken && b == '-'
}
