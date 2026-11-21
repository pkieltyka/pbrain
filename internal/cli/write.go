package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/secretscan"
	"github.com/pkieltyka/pbrain/internal/store"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type itemWrite struct {
	Author, StaleAfter      string
	Tags, Replaces, Relates []string
	Explicit                audience.Destination
}

type writeContext struct {
	Brain  *brain.Brain
	View   brain.View
	Now    time.Time
	Rand   io.Reader
	Stdin  io.Reader
	IsTTY  bool
	Notice func(errs.Notice)
}

type writeResult struct {
	ID, Path  string
	Section   item.Section
	committed bool
}

type personalRoot struct {
	path   string
	id     sys.Identity
	exists bool
}

type writeRoute struct {
	request  audience.Request
	decision audience.Decision
	plan     store.WritePlan
	personal personalRoot
}

type copiedOriginal struct {
	Digest [32]byte
	Bytes  []byte
}

type newItemInput struct {
	route    writeRoute
	draft    item.Draft
	body     []byte
	original *copiedOriginal
}

func (wc writeContext) notify(code, message string) {
	if wc.Notice != nil {
		wc.Notice(errs.Notice{Code: code, Message: message})
	}
}

func writeError(code, message string) *errs.Error {
	return &errs.Error{Code: code, Message: message, Exit: 1}
}

func invalidWrite(message string) *errs.Error { return writeError("invalid", message) }

func forView(decision audience.Decision, disclose brain.View) store.WritePlan {
	plan := store.WritePlan{Disclose: disclose}
	if decision.Dest == audience.Personal {
		plan.Dest = item.Personal
		plan.Lookup = brain.PersonalView
	}
	return plan
}

func routeWrite(wc writeContext, request audience.Request) (writeRoute, error) {
	rules, err := wc.Brain.AudienceRules()
	if err != nil {
		return writeRoute{}, err
	}
	decision, err := audience.Decide(rules, request)
	if err != nil {
		return writeRoute{}, err
	}
	plan := forView(decision, wc.View)
	if err := wc.Brain.RequireView(plan.Lookup); err != nil {
		return writeRoute{}, err
	}
	root, err := snapshotPersonal(wc.Brain)
	if err != nil {
		return writeRoute{}, err
	}
	return writeRoute{request: request, decision: decision, plan: plan, personal: root}, nil
}

func snapshotPersonal(b *brain.Brain) (personalRoot, error) {
	root := personalRoot{path: b.Personal.Dir}
	if !b.Personal.Reachable {
		// A configured directory may resolve but still be inaccessible. Shared
		// commands can proceed with that same unavailable section.
		return resolvePersonal(b, root.path), nil
	}
	dir, err := b.OpenSection(item.Personal)
	if errors.Is(err, fs.ErrNotExist) {
		return root, nil // a personal write can create the default folder under Begin
	}
	if err != nil {
		return personalRoot{}, err
	}
	info, err := b.Files().DirInfo(dir)
	if err != nil {
		return personalRoot{}, ioFailure("could not check the personal section")
	}
	root.id, root.exists = info.Identity, true
	return root, nil
}

// refreshLocal uses the brain's already guarded handle. In particular it does
// not switch a write to a different personal root after taking its lock.
func refreshLocal(b *brain.Brain, before personalRoot, dest item.Section) (brain.LocalConfig, error) {
	dir, err := b.OpenSection(item.Shared)
	if err != nil {
		return brain.LocalConfig{}, err
	}
	local, err := brain.ReadLocal(dir)
	if err != nil {
		return brain.LocalConfig{}, err
	}
	path := local.Path
	if path == "" {
		path = "personal"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.Dir, path)
	}
	after := resolvePersonal(b, filepath.Clean(path))
	if before == after {
		return local, nil
	}
	if !before.exists && after.exists && before.path == after.path &&
		dest == item.Personal && local.Path == "" {
		actual, err := snapshotPersonal(b)
		if err == nil && actual == after {
			return local, nil
		}
	}
	return brain.LocalConfig{}, writeError("conflict", "the personal section changed while preparing the write")
}

func resolvePersonal(b *brain.Brain, path string) personalRoot {
	root := personalRoot{path: path}
	if resolved, err := b.Files().Resolve(path, true); err == nil {
		root.path = resolved.Path
		if resolved.Exists && resolved.Info.Mode.IsDir() {
			root.exists, root.id = true, resolved.Info.Identity
		}
	}
	return root
}

func (route writeRoute) refresh(b *brain.Brain) error {
	local, err := refreshLocal(b, route.personal, route.plan.Dest)
	if err != nil {
		return err
	}
	decision, err := audience.Decide(local.Audience, route.request)
	if err != nil || decision != route.decision {
		return writeError("conflict", "the audience rule changed while preparing the write")
	}
	return nil
}

func (route writeRoute) destinationNotice(wc writeContext, suffix string) {
	section := route.plan.Dest
	root := wc.Brain.Shared.Dir
	distribution := "The item reaches everyone with the repository once committed."
	if section == item.Personal {
		root = wc.Brain.Personal.Dir
		distribution = "The item stays outside Git."
	}
	path := safeDiagnosticValue(wc.Brain.DisplayPath(section, root))
	message := fmt.Sprintf("Writing to %s in %s.", section, path)
	if route.decision.Matched != "" {
		message += " Audience rule " + safeDiagnosticValue(route.decision.Matched) + " selected this section."
	}
	wc.notify("destination", message+" "+distribution+suffix)
}

// safeDiagnosticValue is an id or a path as a message names it: withheld when the secret scan flags
// it, otherwise one token on one line (render.ExactValue).
func safeDiagnosticValue(value string) string {
	if secretscan.Sensitive(value) {
		return withheldValue
	}
	return render.ValueText(value)
}

const withheldValue = "[withheld]"

// valueTexts writes each id of a result list through render.ExactValue; a withheld one stays as it is.
func valueTexts(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id
		if id != withheldValue {
			out[i] = render.ValueText(id)
		}
	}
	return out
}

func visible(section item.Section, view brain.View) bool {
	return section == item.Shared || view == brain.PersonalView
}

// disclosedIDs keeps list ordering and counts within the command's allowed view. With markPersonal
// the ids are for a message: written as tokens and marked personal; otherwise they are the stored
// ids of a result, with a flagged one withheld.
func disclosedIDs(view brain.View, candidates []*item.Item, markPersonal bool) []string {
	items := slices.DeleteFunc(slices.Clone(candidates), func(it *item.Item) bool {
		return !visible(it.Section, view)
	})
	slices.SortStableFunc(items, func(a, b *item.Item) int {
		return cmp.Or(cmp.Compare(a.Key, b.Key), cmp.Compare(a.Path, b.Path))
	})
	var ids []string
	for _, it := range items {
		id := it.ID
		switch {
		case secretscan.Sensitive(id):
			id = withheldValue
		case markPersonal:
			id = render.ValueText(id)
		}
		if markPersonal && it.Section == item.Personal {
			id += " (personal)"
		}
		ids = append(ids, id)
	}
	return ids
}

func shortIDs(ids []string) ([]string, int) {
	n := min(10, len(ids))
	return ids[:n], len(ids) - n
}

func idList(ids []string) string {
	first, more := shortIDs(ids)
	text := strings.Join(first, ", ")
	if more > 0 {
		text += fmt.Sprintf(" (+%d more)", more)
	}
	return text
}

func writeSkipped(wc writeContext, cat *store.Catalog) {
	if cat == nil {
		return
	}
	seen := make(map[string]struct{})
	for _, entry := range cat.Skipped() {
		if visible(entry.Section, wc.View) && broken(entry) {
			seen[entry.Path] = struct{}{}
		}
	}
	if n := len(seen); n > 0 {
		command := "pbrain check"
		if wc.View == brain.PersonalView {
			command += " --personal"
		}
		wc.notify("skipped_files", fmt.Sprintf("%s skipped: run %s", counted(n, "file", "files"), command))
	}
}

func reportCleanup(wc writeContext, commit store.Commit) {
	for _, step := range commit.Cleanup {
		switch step {
		case store.TempLeft:
			wc.notify("after_write", "a temporary item file was left")
		case store.FolderSyncFailed:
			wc.notify("after_write", "the item folder could not be synced")
		}
	}
}

func closeWrite(wc writeContext, tx *store.Tx, committed *bool, resultErr *error) {
	if err := tx.Close(); err != nil {
		if *committed {
			wc.notify("after_write", "the item's write lock or folder handle could not be closed")
		} else if *resultErr == nil {
			*resultErr = ioFailure("the write lock or folder handle could not be closed")
		}
	}
}

func lookupOne(cat *store.Catalog, id string) (*item.Item, error) {
	switch matches := cat.Lookup(id); len(matches) {
	case 0:
		return nil, writeError("not_found", "no item has that id in this view")
	case 1:
		return matches[0], nil
	default:
		return nil, writeError("ambiguous_id", "more than one item has that id in this view")
	}
}

func reselect(cat *store.Catalog, id string, previous *item.Item) (*item.Item, error) {
	it, err := lookupOne(cat, id)
	if err != nil || it.Section != previous.Section || it.Path != previous.Path {
		return nil, writeError("conflict", "the item resolved differently after the write lock was taken")
	}
	return it, nil
}

func writeItem(wc writeContext, it *item.Item, committed bool) writeResult {
	return writeResult{
		ID: it.ID, Section: it.Section, Path: wc.Brain.DisplayPath(it.Section, it.Path), committed: committed,
	}
}

func normalizedDraft(wc writeContext, ctx context.Context, fields itemWrite, draft item.Draft) (item.Draft, error) {
	author, err := wc.Brain.ResolveAuthor(ctx, fields.Author)
	if err != nil {
		return item.Draft{}, err
	}
	draft.Author, draft.Created = author, wc.Now
	draft.Replaces, draft.Relates = fields.Replaces, fields.Relates
	for _, raw := range fields.Tags {
		tag, ok := item.NormTag(raw)
		if !ok {
			return item.Draft{}, invalidWrite("tags: use letters a-z, digits or hyphens")
		}
		draft.Tags = append(draft.Tags, tag)
	}
	if fields.StaleAfter != "" {
		_, normalized, ok := item.ParseStaleAfter(fields.StaleAfter)
		if !ok {
			return item.Draft{}, invalidWrite("stale_after: use a date or RFC 3339 instant")
		}
		draft.StaleAfter = normalized
	}
	if err := draft.Validate(); err != nil {
		return item.Draft{}, err
	}
	return draft, nil
}

func validateLinks(plan store.WritePlan, cat *store.Catalog, g *graph.Graph, draft *item.Draft) ([]*item.Item, error) {
	header := item.Header{Kind: draft.Kind, Status: draft.Status, Decided: draft.Decided}
	var replaced []*item.Item
	for _, id := range draft.Replaces {
		items, err := g.CheckNewReplaces(plan.Dest, &header, []string{id})
		if err != nil {
			e := domainError(err)
			message := e.Message
			var refused *graph.LinkError
			if errors.As(err, &refused) {
				message = "replaces " + safeDiagnosticValue(refused.Target) + ": " + refused.Reason
			}
			for _, it := range cat.Lookup(id) {
				if !visible(it.Section, plan.Disclose) {
					message = "replacement target refused in the selected section"
					break
				}
			}
			if ids := disclosedIDs(plan.Disclose, items, true); len(ids) > 0 {
				message += ". Current successors: " + idList(ids)
			}
			return nil, writeError(e.Code, message)
		}
		replaced = append(replaced, items...)
	}
	for _, id := range draft.Relates {
		if _, err := lookupOne(cat, id); err != nil {
			return nil, writeError(domainError(err).Code, "could not resolve a related item")
		}
	}
	return replaced, nil
}

func checkDuplicates(plan store.WritePlan, cat *store.Catalog, g *graph.Graph, draft *item.Draft, replaces []*item.Item) error {
	if draft.Kind != item.Source || draft.Origin == item.OriginNone {
		return nil
	}
	var duplicates []*item.Item
	for _, it := range cat.Items() {
		h := &it.Header
		if it.Section != plan.Dest || h.Kind != item.Source || !g.Current(it) || slices.Contains(replaces, it) {
			continue
		}
		fileDuplicate := draft.Origin == item.OriginFile && h.Origin == item.OriginFile && draft.SHA256 == h.SHA256
		urlDuplicate := draft.Origin != item.OriginFile && (h.Origin == item.OriginURL || h.Origin == item.OriginLink) && draft.Source == h.Source
		if fileDuplicate || urlDuplicate {
			duplicates = append(duplicates, it)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	message := "a current source already records this input"
	if ids := disclosedIDs(plan.Disclose, duplicates, true); len(ids) > 0 {
		message += ": " + idList(ids)
	}
	return writeError("duplicate", message)
}

func occupiedKeys(cat *store.Catalog, dest item.Section) map[string]bool {
	keys := make(map[string]bool, len(cat.Items()))
	for _, it := range cat.Items() {
		keys[it.Key] = true
		if it.Section == dest {
			for _, id := range it.Header.Replaces {
				keys[item.Fold(id)] = true
			}
			if id, ok := it.Header.LegacyStamp(); ok {
				keys[item.Fold(id)] = true
			}
		}
	}
	return keys
}

func publishNew(ctx context.Context, wc writeContext, input newItemInput) (result writeResult, err error) {
	plan := input.route.plan
	payload, err := item.Emit(&input.draft, input.body)
	if err != nil {
		return writeResult{}, err
	}
	if len(payload) > store.MaxItemBytes {
		return writeResult{}, writeError("too_large", "the item file would exceed 4 MiB")
	}
	tx, err := store.Begin(ctx, wc.Brain, plan.Dest)
	if err != nil {
		return writeResult{}, err
	}
	defer closeWrite(wc, tx, &result.committed, &err)
	if err := input.route.refresh(wc.Brain); err != nil {
		return writeResult{}, err
	}
	cat, err := store.Scan(ctx, wc.Brain, plan.Lookup, store.KeepAll)
	if err != nil {
		return writeResult{}, err
	}
	writeSkipped(wc, cat)
	g := graph.Build(cat.Items())
	replaced, err := validateLinks(plan, cat, g, &input.draft)
	if err != nil {
		return writeResult{}, err
	}
	if err := checkDuplicates(plan, cat, g, &input.draft, replaced); err != nil {
		return writeResult{}, err
	}
	warnNewItem(wc, plan, &input.draft, input.body)
	occupied := occupiedKeys(cat, plan.Dest)
	original := input.original
	if wc.Rand == nil {
		return writeResult{}, ioFailure("could not draw a random id")
	}
	for range 16 {
		id, err := item.NewID(wc.Now, input.draft.Title, wc.Rand, func(key string) bool { return occupied[key] })
		if err != nil {
			return writeResult{}, err
		}
		if err := tx.CheckDestination(ctx, input.draft.Kind, id); err != nil {
			if errors.Is(err, store.ErrNameExists) {
				occupied[item.Fold(id)] = true
				continue
			}
			return writeResult{}, err
		}
		if original != nil {
			if err := tx.PutOriginal(original.Digest, original.Bytes); err != nil {
				return writeResult{}, err
			}
			original = nil
		}
		commit, err := tx.Publish(ctx, input.draft.Kind, id, payload)
		if errors.Is(err, store.ErrNameExists) {
			occupied[item.Fold(id)] = true
			continue
		}
		if err != nil {
			return writeResult{}, err
		}
		root := wc.Brain.Shared.Dir
		if plan.Dest == item.Personal {
			root = wc.Brain.Personal.Dir
		}
		path := filepath.Join(root, input.draft.Kind.String()+"s", id+".md")
		reportCleanup(wc, commit)
		return writeResult{ID: id, Path: wc.Brain.DisplayPath(plan.Dest, path), Section: plan.Dest, committed: true}, nil
	}
	return writeResult{}, ioFailure("could not publish under an unused id")
}
