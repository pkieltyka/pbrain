package cli

import (
	"context"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/secretscan"
	"github.com/pkieltyka/pbrain/internal/store"
)

type verifyRequest struct{ ID, By string }

func runVerify(ctx context.Context, wc writeContext, req verifyRequest) (result writeResult, err error) {
	if _, err := wc.Brain.AudienceRules(); err != nil {
		return writeResult{}, err
	}
	root, err := snapshotPersonal(wc.Brain)
	if err != nil {
		return writeResult{}, err
	}
	cat, err := store.Scan(ctx, wc.Brain, wc.View, store.KeepAll)
	if err != nil {
		return writeResult{}, err
	}
	defer func() { writeSkipped(wc, cat) }()
	first, err := lookupOne(cat, req.ID)
	if err != nil {
		return writeResult{}, err
	}
	tx, err := store.Begin(ctx, wc.Brain, first.Section)
	if err != nil {
		return writeResult{}, err
	}
	defer closeWrite(wc, tx, &result.committed, &err)
	if _, err := refreshLocal(wc.Brain, root, first.Section); err != nil {
		return writeResult{}, err
	}
	fresh, err := store.Scan(ctx, wc.Brain, wc.View, store.KeepAll)
	if err != nil {
		return writeResult{}, err
	}
	cat = fresh
	it, err := reselect(cat, req.ID, first)
	if err != nil {
		return writeResult{}, err
	}
	if it.Version != first.Version {
		return writeResult{}, writeError("conflict", "the item changed since it was read")
	}
	switch {
	case it.Header.Kind == item.Source:
		return writeResult{}, invalidWrite("a source cannot be verified")
	case !graph.Build(cat.Items()).Current(it):
		return writeResult{}, invalidWrite("a replaced item cannot be verified")
	case it.Header.Status == item.Verified:
		return writeItem(wc, it, false), nil
	}
	actor := req.By
	if actor == "" {
		actor, err = wc.Brain.ResolveAuthor(ctx, "")
		if err != nil {
			return writeResult{}, err
		}
	}
	data, observed, err := tx.ReadFile(it)
	if err != nil {
		return writeResult{}, err
	}
	if observed != first.Version {
		return writeResult{}, writeError("conflict", "the item changed since it was read")
	}
	edited, err := item.EditVerify(data, actor)
	if err != nil {
		return writeResult{}, err
	}
	if len(edited) > store.MaxItemBytes {
		return writeResult{}, writeError("too_large", "the item file would exceed 4 MiB")
	}
	if it.Section == item.Shared {
		warnSecrets(wc, []secretscan.Field{{Name: "verified", Value: actor}})
	}
	commit, err := tx.ReplaceItem(it, first.Version, edited)
	if err != nil {
		return writeResult{}, err
	}
	reportCleanup(wc, commit)
	return writeItem(wc, it, true), nil
}
