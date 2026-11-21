package audience

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/pkieltyka/pbrain/internal/errs"
)

type hostPolicy struct {
	host  string
	paths []string
}

// A leading dot matches workspace subdomains. Path placeholders stand for
// one whole segment. Admission checks use the DNS name irrespective of port.
var mixed = []hostPolicy{
	{"docs.google.com", []string{"/document/d/<id>", "/spreadsheets/d/<id>", "/presentation/d/<id>"}},
	{"drive.google.com", []string{"/file/d/<id>", "/drive/folders/<id>"}},
	{"claude.ai", []string{"/artifact/<id>", "/code/artifact/<id>"}},
	{".slack.com", []string{"/archives/<C-channel-id>"}},
	{"github.com", []string{"/<owner>"}},
	{"gitlab.com", []string{"/<owner>"}},
}

func policy(host string) *hostPolicy {
	for i := range mixed {
		name := mixed[i].host
		if host == name || strings.HasPrefix(name, ".") && strings.HasSuffix(host, name) {
			return &mixed[i]
		}
	}
	return nil
}

func permits(segments []string, pattern string) bool {
	for i, part := range strings.Split(strings.TrimPrefix(pattern, "/"), "/") {
		if i >= len(segments) {
			return false
		}
		switch part {
		case "<id>", "<owner>": // Canon guarantees nonempty segments.
		case "<C-channel-id>":
			if !strings.HasPrefix(segments[i], "C") {
				return false
			}
		default:
			if segments[i] != part {
				return false
			}
		}
	}
	return true
}

func problem(code, message string) *errs.Error {
	exit := 1
	if code == "usage" {
		exit = 2
	}
	return &errs.Error{Code: code, Message: message, Exit: exit}
}

func canonicalRule(match string, to Destination) (Rule, error) {
	if to != Shared && to != Personal {
		return Rule{}, problem("usage", "choose shared or personal")
	}
	k, err := Canon(match, true)
	if err != nil {
		return Rule{}, problem("invalid", "invalid audience rule key")
	}
	if p := policy(k.Host); to == Shared && p != nil {
		if !slices.ContainsFunc(p.paths, func(s string) bool { return permits(k.Segments, s) }) {
			return Rule{}, problem("invalid", "a shared rule for "+k.Host+" must start with "+strings.Join(p.paths, " or "))
		}
	}
	return Rule{Match: k.String(), To: to}, nil
}

// ValidateRules returns a sorted, normalized copy. Diagnostics point to the
// original table position even if its canonical order would be different.
func ValidateRules(rules []Rule) ([]Rule, error) {
	result := make([]Rule, 0, len(rules))
	seen := make(map[string]bool, len(rules))
	for i, entry := range rules {
		r, err := canonicalRule(entry.Match, entry.To)
		if err != nil || seen[r.Match] {
			return nil, problem("invalid_config", fmt.Sprintf("invalid audience rule at position %d; edit personal.toml by hand", i+1))
		}
		result = append(result, r)
		seen[r.Match] = true
	}
	slices.SortFunc(result, func(a, b Rule) int { return cmp.Compare(a.Match, b.Match) })
	return result, nil
}

func position(rules []Rule, key string) (int, bool) {
	return slices.BinarySearchFunc(rules, key, func(r Rule, key string) int { return cmp.Compare(r.Match, key) })
}

func Set(rules []Rule, match string, to Destination) ([]Rule, error) {
	table, err := ValidateRules(rules)
	if err != nil {
		return nil, err
	}
	r, err := canonicalRule(match, to)
	if err != nil {
		return nil, err
	}
	i, found := position(table, r.Match)
	if found {
		table[i] = r
		return table, nil
	}
	return slices.Insert(table, i, r), nil
}

func Unset(rules []Rule, match string) ([]Rule, error) {
	table, err := ValidateRules(rules)
	if err != nil {
		return nil, err
	}
	k, err := Canon(match, true)
	if err != nil {
		return nil, problem("invalid", "invalid audience rule key")
	}
	if i, found := position(table, k.String()); found {
		return slices.Delete(table, i, i+1), nil
	}
	return table, nil
}

// Decide is pure, so callers can repeat it with fresh settings under the lock.
func Decide(rules []Rule, req Request) (Decision, error) {
	table, err := ValidateRules(rules)
	if err != nil {
		return Decision{}, err
	}
	switch req.Explicit {
	case Shared, Personal:
		return Decision{Dest: req.Explicit}, nil
	case "":
	default:
		return Decision{}, problem("usage", "choose shared or personal")
	}
	source := strings.Trim(req.Source, " \t")
	if req.Method == Citation {
		scheme, _, found := strings.Cut(source, ":")
		if !found || !(len(scheme) == 4 && strings.EqualFold(scheme, "http") || len(scheme) == 5 && strings.EqualFold(scheme, "https")) {
			return Decision{Dest: Shared}, nil
		}
	}
	k, err := Canon(source, false)
	if err != nil {
		return Decision{}, problem("audience_required", "choose --shared or --personal for this source")
	}
	key := k.String()
	var winner Rule
	for _, r := range table {
		if (key == r.Match || strings.HasPrefix(key, r.Match+"/")) && len(r.Match) > len(winner.Match) {
			winner = r
		}
	}
	if winner.Match != "" {
		return Decision{Dest: winner.To, Matched: winner.Match}, nil
	}
	if req.Method == Citation || req.Method == LinkedURL && policy(k.Host) != nil {
		return Decision{}, problem("audience_required", "choose --shared or --personal for "+k.Host)
	}
	return Decision{Dest: Shared}, nil
}
