package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/cli"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type response struct {
	out, diagnostic []byte
	elapsed         time.Duration
}

type searchPage struct {
	Results []struct {
		ID     string `json:"id"`
		Offset *int   `json:"snippet_offset"`
	} `json:"results"`
	Alone []struct {
		Clause string `json:"clause"`
		Count  int    `json:"count"`
	} `json:"alone"`
}

type attempt struct {
	page  searchPage
	calls []response // The last call supplied page.
}

type session struct {
	ctx    context.Context
	dir    string
	corpus corpus
	model  *modelRun
}

func (s session) command(args ...string) (response, error) {
	var out, diag bytes.Buffer
	env := cli.Env{
		Args: args, Dir: s.dir, Stdout: &out, Stderr: &diag, Files: sys.Native(),
		Now: func() time.Time { return frozenTime },
		Getenv: func(key string) string {
			if key == "TZ" {
				return "UTC"
			}
			if key == "PBRAIN_CACHE" && s.model != nil {
				return s.model.cache
			}
			return ""
		},
	}
	if s.model != nil {
		env.Models = s.model.registry
	}
	start := time.Now()
	status := cli.RunWith(s.ctx, env, cli.Features{BM25: true})
	got := response{out: out.Bytes(), diagnostic: diag.Bytes(), elapsed: time.Since(start)}
	if status != 0 {
		return got, fmt.Errorf("pbrain %q exited %d: %s%s", args, status, got.diagnostic, got.out)
	}
	return got, nil
}

func (s session) search(text string, bm25 bool) (attempt, error) {
	return s.searchMode(text, bm25, false)
}

func (s session) searchMode(text string, bm25, semantic bool) (attempt, error) {
	args := []string{"search", "--json"}
	if bm25 {
		args = append(args, "--bm25")
	}
	if semantic {
		args = append(args, "--semantic")
	}
	got, err := s.command(append(args, "--", text)...)
	if err != nil {
		return attempt{}, err
	}
	if semantic && (bytes.Contains(got.diagnostic, []byte(`"semantic_not_ready"`)) ||
		bytes.Contains(got.diagnostic, []byte(`"semantic_partial"`))) {
		return attempt{}, fmt.Errorf("cannot gate an incomplete semantic search: %s", got.diagnostic)
	}
	return decodePage(got)
}

func decodePage(got response) (attempt, error) {
	var page searchPage
	if err := json.Unmarshal(got.out, &page); err != nil {
		return attempt{}, err
	}
	return attempt{page: page, calls: []response{got}}, nil
}

func (s session) retry(text string) (attempt, error) {
	first, err := s.search(text, false)
	if err != nil || len(first.page.Results) != 0 {
		return first, err
	}
	q, err := query.Parse(text)
	if err != nil {
		return attempt{}, err
	}
	var words []string
	for _, clause := range q {
		for _, hint := range first.page.Alone {
			if hint.Count > 0 && clause.Label == hint.Clause {
				words = append(words, clause.Label)
				break
			}
		}
	}
	if len(words) == 0 {
		return first, nil
	}
	second, err := s.search(strings.Join(words, " "), false)
	if err != nil {
		return attempt{}, err
	}
	second.calls = append(first.calls, second.calls...)
	return second, nil
}

func (s session) get(id string, offset *int, personal bool) (response, error) {
	args := []string{"get", "--json"}
	if personal {
		args = append(args, "--personal")
	}
	if offset != nil {
		args = append(args, "--offset", strconv.Itoa(*offset))
	}
	return s.command(append(args, "--", id)...)
}

type evidenceRead struct {
	rank       int
	onPage     bool
	afterRead  bool
	upperBound bool
	toEvidence int
	delivered  int
	followUps  []response // Actual follow-up, then the separate oracle's read if different.
}

func (s session) evidence(q question, a attempt) (evidenceRead, error) {
	var result evidenceRead
	if len(a.page.Results) == 0 {
		return result, nil
	}
	first := a.page.Results[0]
	actual, err := s.get(first.ID, first.Offset, false)
	if err != nil {
		return result, err
	}
	result.followUps = append(result.followUps, actual)
	for i, hit := range a.page.Results {
		if hit.ID == q.Expect[0] {
			result.rank = i + 1
			break
		}
	}
	if result.rank == 0 {
		return result, nil
	}
	last := a.calls[len(a.calls)-1].out
	end, err := evidenceEnd(last, result.rank-1, "snippet", q.Evidence)
	if err != nil {
		return result, err
	}
	switch {
	case end > 0:
		result.onPage, result.upperBound = true, true
		result.toEvidence = responseChars(a.calls[:len(a.calls)-1]) + end
		result.delivered = responseChars(a.calls)
	case result.rank == 1:
		end, err = evidenceEnd(actual.out, -1, "body", q.Evidence)
		if err != nil {
			return result, err
		}
		if end != 0 {
			result.afterRead, result.upperBound = true, true
			result.toEvidence = responseChars(a.calls) + end
			result.delivered = responseChars(a.calls) + utf8.RuneCount(actual.out)
		}
	default:
		hit := a.page.Results[result.rank-1]
		oracle, err := s.get(hit.ID, hit.Offset, false)
		if err != nil {
			return result, err
		}
		result.followUps = append(result.followUps, oracle)
		end, err := evidenceEnd(oracle.out, -1, "body", q.Evidence)
		if err != nil {
			return result, err
		}
		result.upperBound = end > 0
	}
	return result, err
}

func responseChars(calls []response) (n int) {
	for _, call := range calls {
		n += utf8.RuneCount(call.out)
	}
	return n
}

// evidenceEnd locates the end of the quote in the exact JSON the reader saw.
// Return zero when the given row/field does not show the evidence.
func evidenceEnd(out []byte, hit int, field, quote string) (int, error) {
	root := out
	rowStart := 0
	if hit >= 0 {
		rows, start, err := fieldBytes(out, "results")
		if err != nil {
			return 0, err
		}
		d := json.NewDecoder(bytes.NewReader(rows))
		if _, err := d.Token(); err != nil {
			return 0, err
		}
		var row json.RawMessage
		for range hit + 1 {
			if err := d.Decode(&row); err != nil {
				return 0, err
			}
		}
		rowStart = start + int(d.InputOffset()) - len(row)
		root = row
	}
	literal, fieldStart, err := fieldBytes(root, field)
	if err != nil || literal == nil {
		return 0, err
	}
	var value string
	if err := json.Unmarshal(literal, &value); err != nil {
		return 0, err
	}
	at := quoteEnd(value, normalize(quote))
	if at == 0 {
		return 0, nil
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value[:at]); err != nil {
		return 0, err
	}
	prefix := bytes.TrimSuffix(b.Bytes(), []byte("\"\n")) // omit the closing quote and encoder newline
	if !bytes.HasPrefix(literal, prefix) {
		return 0, errors.New("evidence field did not use pbrain's JSON encoding")
	}
	return utf8.RuneCount(out[:rowStart+fieldStart]) + utf8.RuneCount(prefix), nil
}

func fieldBytes(obj []byte, field string) (json.RawMessage, int, error) {
	d := json.NewDecoder(bytes.NewReader(obj))
	if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
		return nil, 0, errors.New("expected a JSON object")
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, 0, err
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, 0, err
		}
		if key == field {
			return value, int(d.InputOffset()) - len(value), nil
		}
	}
	return nil, 0, nil
}

// Successive fields may be separated by any Unicode whitespace in a body.
// Match that equivalence while retaining the position in the source string.
func quoteEnd(value, quote string) int {
	if quote == "" {
		return 0
	}
	words := strings.Split(quote, " ")
	pattern := make([]string, len(words))
	for i, word := range words {
		pattern[i] = regexp.QuoteMeta(word)
	}
	// Unicode IsSpace is Latin-1 space plus the Z category and Unicode NEL.
	ws := `[\x09-\x0d\x{85}\p{Z}]+`
	match := regexp.MustCompile(strings.Join(pattern, ws)).FindStringIndex(value)
	if match == nil {
		return 0
	}
	return match[1]
}
