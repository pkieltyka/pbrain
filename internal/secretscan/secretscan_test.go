package secretscan

import (
	"slices"
	"strings"
	"testing"
)

func TestSecretAndTagNotices(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----"
	aws := "AK" + "IA" + strings.Repeat("Q", 16)
	github := "gh" + "p_" + strings.Repeat("v", 36)
	pat := "github" + "_pat_" + strings.Repeat("x_", 10)
	slack := "xo" + "xb-" + strings.Repeat("a", 10)
	fields := []Field{
		{Name: "title", Value: "source example"},
		{Name: "tags", Value: "redis\n" + slack},
		{Name: "body", Value: "<" + pem + ">\r\n" + github + "," + pat + ":" + aws + ":" + aws},
		{Name: "verified", Value: "(agent:" + slack + ")"},
	}
	want := []Match{
		{Field: "tags", Line: 2, Pattern: SlackToken},
		{Field: "body", Line: 1, Pattern: PEMPrivateKey},
		{Field: "body", Line: 2, Pattern: AWSAccessKey},
		{Field: "body", Line: 2, Pattern: GitHubToken},
		{Field: "verified", Line: 1, Pattern: SlackToken},
	}
	if got := Scan(fields); !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	for _, tc := range []struct {
		name, value string
		pattern     Pattern
	}{
		{"adjacent tokens separated by punctuation", aws + "," + github + "," + pat, AWSAccessKey},
		{"AWS reply type ASIA", "AS" + "IA" + strings.Repeat("1", 16), AWSAccessKey},
		{"short PAT", pat, GitHubToken},
		{"Slack tag exactly", slack, SlackToken},
		{"embedded AWS", "long" + aws, ""},
		{"extended AWS", aws + "extra", ""},
		{"embedded GitHub", "_" + github, ""},
		{"mixed case GitHub", github[:4] + strings.Repeat("aZ0", 12), GitHubToken},
		{"extended GitHub", github + "_", ""},
		{"long PAT", "github" + "_pat_" + strings.Repeat("A", 256), ""},
		{"embedded Slack", "tag-" + slack, ""},
		{"extended Slack", slack + "_", ""},
		{"PEM marker punctuation", "x" + pem + "z", PEMPrivateKey},
		{"missing Slack body", "xo" + "xb-example", ""},
		{"token split on lines", github[:16] + "\n" + github[16:], ""},
		{"public PEM marker", "-----BEGIN PUBLIC KEY-----", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matches := Scan([]Field{{Name: "value", Value: tc.value}})
			if (len(matches) != 0) != Sensitive(tc.value) || Sensitive(tc.value) != (tc.pattern != "") {
				t.Fatalf("Scan and Sensitive disagree with expected %q: %+v", tc.pattern, matches)
			}
			if tc.pattern != "" && matches[0].Pattern != tc.pattern {
				t.Errorf("first match is %+v, want %q", matches[0], tc.pattern)
			}
		})
	}
}

func TestScanRetainsFieldAndLineOrder(t *testing.T) {
	pem := "-----BEGIN PRIVATE KEY-----"
	input := []Field{
		{Name: "z", Value: "\n" + pem + "\n"},
		{Name: "z", Value: pem},
		{Name: "a", Value: pem + " " + pem},
	}
	want := []Match{{"z", 2, PEMPrivateKey}, {"z", 1, PEMPrivateKey}, {"a", 1, PEMPrivateKey}}
	if got := Scan(input); !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
