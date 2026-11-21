package cli

import (
	"context"
	"fmt"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/store"
)

func runAudienceCommand(ctx context.Context, env Env, in *invocation, b *brain.Brain) (out outcome) {
	rules, err := b.AudienceRules()
	if err != nil {
		return outcome{err: err}
	}
	if len(in.operands) == 0 {
		return listAudience(in, rules)
	}

	key, err := audience.Canon(in.operands[1], true)
	if err != nil {
		return outcome{err: invalidWrite("invalid audience rule key")}
	}
	match := key.String()
	var to audience.Destination
	if in.operands[0] == "set" {
		to = audience.Destination(in.operands[2])
	}
	dir, err := b.OpenSection(item.Shared)
	if err != nil {
		return outcome{err: err}
	}
	tx, err := store.Begin(ctx, b, item.Shared)
	if err != nil {
		return outcome{err: err}
	}
	defer func() {
		if err := tx.Close(); err != nil && out.err == nil {
			out.err = ioFailure("the settings lock or folder handle could not be closed")
		}
	}()
	data, changed, err := brain.EditAudience(dir, func(current []audience.Rule) ([]audience.Rule, error) {
		if to == "" {
			return audience.Unset(current, match)
		}
		return audience.Set(current, match, to)
	})
	if err != nil {
		return outcome{err: err}
	}
	if changed {
		commit, err := tx.ReplaceSettings(data)
		if err != nil {
			return outcome{err: err}
		}
		for _, step := range commit.Cleanup {
			var message string
			switch step {
			case store.TempLeft:
				message = "a temporary settings file was left"
			case store.FolderSyncFailed:
				message = "the settings folder could not be synced"
			}
			emitNotice(env.Stderr, in.json, errs.Notice{Code: "after_write", Message: message})
		}
	}
	line := match
	fields := render.Object{"match": match}
	if to != "" {
		line += "  " + string(to)
		fields["to"] = to
	}
	out = textOrJSON(in.json, line+"\n", fields)
	out.committed = changed
	return out
}

func listAudience(in *invocation, rules []audience.Rule) outcome {
	d := render.Doc{
		Head:  []render.Part{{JSON: render.Envelope{Rows: "rules"}}},
		Total: len(rules), Offset: in.page.offset,
		Foot: func(st render.State) render.Part {
			p := render.Part{Text: fmt.Sprintf("%d rules.", st.Shown)}
			if st.More > 0 {
				p.Text += fmt.Sprintf(" %d more: --offset %d", st.More, st.Next)
				p.JSON = render.Object{"more": st.More, "next": st.Next}
			}
			return p
		},
	}
	start := min(in.page.offset, len(rules))
	count := min(in.page.limit, len(rules)-start)
	for _, r := range rules[start : start+count] {
		d.Units = append(d.Units, render.Unit{Main: render.Part{
			Text: r.Match + "  " + string(r.To), JSON: render.Object{"match": r.Match, "to": r.To},
		}})
	}
	data, err := render.Fit(&d, in.page.maxChars, in.format)
	return outcome{data: data, err: err}
}
