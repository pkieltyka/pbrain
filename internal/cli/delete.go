package cli

import (
	"context"
	"encoding/hex"

	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/store"
)

type deleteRequest struct {
	ID  string
	Yes bool
}

type deleteResult struct {
	Item                   writeResult
	Deleted                bool
	CurrentAgain, Dangling []string
	Original               string
}

func runDelete(ctx context.Context, wc writeContext, req deleteRequest) (result deleteResult, err error) {
	if _, err := wc.Brain.AudienceRules(); err != nil {
		return deleteResult{}, err
	}
	cat, err := store.Scan(ctx, wc.Brain, wc.View, store.KeepAll)
	if err != nil {
		return deleteResult{}, err
	}
	defer func() { writeSkipped(wc, cat) }()
	first, err := lookupOne(cat, req.ID)
	if err != nil {
		return deleteResult{}, err
	}
	if !req.Yes {
		facts, _ := previewDelete(wc, cat, first)
		return facts, writeError("confirm_required", "Nothing changed. Repeat with --yes to delete.")
	}

	root, err := snapshotPersonal(wc.Brain)
	if err != nil {
		return deleteResult{}, err
	}
	tx, err := store.Begin(ctx, wc.Brain, first.Section)
	if err != nil {
		return deleteResult{}, err
	}
	defer closeWrite(wc, tx, &result.Deleted, &err)
	if _, err := refreshLocal(wc.Brain, root, first.Section); err != nil {
		return deleteResult{}, err
	}
	fresh, err := store.Scan(ctx, wc.Brain, wc.View, store.KeepAll)
	if err != nil {
		return deleteResult{}, err
	}
	cat = fresh
	it, err := reselect(cat, req.ID, first)
	if err != nil {
		return deleteResult{}, err
	}
	facts, digest := previewDelete(wc, cat, it)
	commit, err := tx.RemoveItem(it)
	if err != nil {
		return deleteResult{}, err
	}
	facts.Deleted = true
	reportCleanup(wc, commit)
	switch facts.Original {
	case "removed":
		removed, err := tx.RemoveOriginal(digest)
		if !removed {
			facts.Original = "left"
			wc.notify("after_write", "the original could not be removed")
		} else if err != nil {
			wc.notify("after_write", "the originals folder could not be synced")
		}
	case "left":
		wc.notify("after_write", "the original could not be removed safely")
	}
	if it.Section == item.Shared {
		wc.notify("git_history", "If the file was committed, commit the deletion. Git history still holds it.")
	}
	return facts, nil
}

func previewDelete(wc writeContext, cat *store.Catalog, it *item.Item) (deleteResult, [32]byte) {
	impact := graph.Effect(cat.Items(), it)
	status, digest := originalDisposition(wc, cat, it)
	return deleteResult{
		Item: writeItem(wc, it, false), Original: status,
		CurrentAgain: disclosedIDs(wc.View, impact.CurrentAgain, false),
		Dangling:     disclosedIDs(wc.View, impact.Dangling, false),
	}, digest
}

func originalDisposition(wc writeContext, cat *store.Catalog, it *item.Item) (string, [32]byte) {
	var digest [32]byte
	h := &it.Header
	if h.Kind != item.Source || h.Origin != item.OriginFile && h.Origin != item.OriginURL {
		return "", digest
	}
	if !validDigest(h.SHA256) {
		return "left", digest
	}
	if _, err := hex.Decode(digest[:], []byte(h.SHA256)); err != nil {
		return "left", digest
	}
	for _, other := range cat.Items() {
		if other != it && other.Section == it.Section && other.Header.SHA256 == h.SHA256 {
			return "kept", digest
		}
	}
	root, err := wc.Brain.OpenSection(it.Section)
	if err != nil {
		return "left", digest
	}
	data, err := root.Sub(".pbrain", false)
	if err != nil {
		return "left", digest
	}
	defer data.Close()
	dir, err := data.Sub("originals", false)
	if err != nil {
		return "left", digest
	}
	defer dir.Close()
	info, err := wc.Brain.Files().Stat(dir, h.SHA256)
	if err != nil || !info.Mode.IsRegular() {
		return "left", digest
	}
	return "removed", digest
}

func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}
