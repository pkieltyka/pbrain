package cli

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/render"
)

func newWriteContext(env Env, in *invocation, b *brain.Brain) writeContext {
	return writeContext{
		Brain: b, View: in.view, Now: in.now, Rand: env.Rand, Stdin: env.Stdin, IsTTY: env.Stdin == nil,
		Notice: func(n errs.Notice) { emitNotice(env.Stderr, in.json, n) },
	}
}

func (in *invocation) writeFields() (itemWrite, error) {
	f := itemWrite{
		Author: in.string("author"), StaleAfter: in.string("stale-after"),
		Tags: in.all("tag"), Replaces: in.all("replaces"), Relates: in.all("relates"),
	}
	if in.has("author") && f.Author == "" {
		return f, invalidWrite("author: must not be empty")
	}
	if in.has("stale-after") && f.StaleAfter == "" {
		return f, invalidWrite("stale_after: must not be empty")
	}
	if in.boolean("shared") {
		f.Explicit = audience.Shared
	} else if in.boolean("personal") {
		f.Explicit = audience.Personal
	}
	return f, nil
}

func runAddCommand(ctx context.Context, in *invocation, wc writeContext) outcome {
	fields, err := in.writeFields()
	if err != nil {
		return outcome{err: err}
	}
	req := addRequest{
		itemWrite: fields, Kind: item.Note, Title: in.string("title"), Source: in.string("source"),
		Decided: in.string("decided"), Verified: in.string("verified"),
	}
	if in.has("kind") {
		kind, ok := item.ParseKind(in.string("kind"))
		if !ok {
			return outcome{err: invalidWrite("kind: add accepts decision, learning, observation or note")}
		}
		req.Kind = kind
	}
	if in.has("decided") && req.Decided == "" || in.has("verified") && req.Verified == "" {
		return outcome{err: invalidWrite("an actor must not be empty")}
	}
	if in.has("body") {
		body := in.string("body")
		req.Body = &body
	}
	if in.has("body-file") {
		file := in.string("body-file")
		req.BodyFile = &file
	}
	r, err := runAdd(ctx, wc, req)
	return writeOutcome(in.json, r, err)
}

// writeOutput is a write's or a delete's result. JSON carries the id and the path as stored, and no
// path that is not valid UTF-8; text writes each through render.ExactValue (spec 6.2).
type writeOutput struct {
	ID      string `json:"id"`
	Section string `json:"section"`
	Path    string `json:"path,omitempty"`
	text    string
}

func displayWrite(r writeResult) writeOutput {
	out := writeOutput{ID: r.ID, Section: r.Section.String(), Path: jsonPath(r.Path)}
	out.text = render.ValueText(r.ID) + "  " + out.Section + "  " + render.ValueText(r.Path) + "\n"
	return out
}

// jsonPath is a path as JSON carries it: "" (left out) when it is not valid UTF-8.
func jsonPath(p string) string {
	if !utf8.ValidString(p) {
		return ""
	}
	return p
}

func (r writeOutput) line() string { return r.text }

func writeOutcome(jsonMode bool, r writeResult, err error) outcome {
	if err != nil {
		return outcome{err: err}
	}
	val := displayWrite(r)
	out := textOrJSON(jsonMode, val.line(), val)
	out.committed = r.committed
	return out
}

type deleteOutput struct {
	writeOutput
	Deleted          bool     `json:"deleted,omitempty"`
	CurrentAgain     []string `json:"current_again,omitempty"`
	CurrentAgainMore int      `json:"current_again_more,omitempty"`
	Dangling         []string `json:"dangling,omitempty"`
	DanglingMore     int      `json:"dangling_more,omitempty"`
	Original         string   `json:"original,omitempty"`
}

func deleteOutcome(jsonMode bool, r deleteResult, err error) outcome {
	if err != nil && domainError(err).Code != "confirm_required" {
		return outcome{err: err}
	}
	firstCurrent, moreCurrent := shortIDs(r.CurrentAgain)
	firstDangling, moreDangling := shortIDs(r.Dangling)
	value := deleteOutput{
		writeOutput: displayWrite(r.Item), Deleted: r.Deleted,
		CurrentAgain: firstCurrent, CurrentAgainMore: moreCurrent,
		Dangling: firstDangling, DanglingMore: moreDangling, Original: jsonPath(r.Original),
	}
	var text strings.Builder
	if r.Deleted {
		text.WriteString("deleted: ")
	} else {
		text.WriteString("would delete: ")
	}
	text.WriteString(value.line())
	if len(r.CurrentAgain) > 0 {
		fmt.Fprintf(&text, "current again: %s\n", idList(valueTexts(r.CurrentAgain)))
	}
	if len(r.Dangling) > 0 {
		fmt.Fprintf(&text, "left dangling: %s\n", idList(valueTexts(r.Dangling)))
	}
	if r.Original != "" {
		fmt.Fprintf(&text, "original: %s\n", render.ValueText(r.Original))
	}
	out := textOrJSON(jsonMode, text.String(), value)
	out.committed = r.Deleted
	if out.err == nil {
		out.err = err
	}
	return out
}
