package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/importer"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/store"
)

func runImportCommand(ctx context.Context, in *invocation, wc writeContext) outcome {
	fields, err := in.writeFields()
	if err != nil {
		return outcome{err: err}
	}
	input := in.operands[0]
	title, source := in.string("title"), in.string("source")
	if in.has("title") && strings.TrimSpace(title) == "" {
		return outcome{err: invalidWrite("title: must not be empty")}
	}
	if in.has("source") && strings.TrimSpace(source) == "" {
		return outcome{err: invalidWrite("source: must not be empty")}
	}
	req := audience.Request{Explicit: fields.Explicit, Source: source, Method: audience.Citation}
	if in.boolean("link-only") {
		req.Method, req.Source = audience.LinkedURL, input
	} else if isWebSource(input) {
		return notImplemented()
	}
	route, err := routeWrite(wc, req)
	if err != nil {
		return outcome{err: err}
	}

	var p importer.Prepared
	if req.Method == audience.LinkedURL {
		suffix := ""
		if route.plan.Dest == item.Shared {
			suffix = " The title, link, tags and summary reach everyone; only the content stays at the source."
		}
		route.destinationNotice(wc, suffix)
		var summary *string
		if in.has("summary") {
			s := in.string("summary")
			summary = &s
		}
		p, err = importer.Link(input, title, summary)
	} else {
		route.destinationNotice(wc, " A copy of the original stays in local data.")
		raw, e := store.OpenInput(wc.Brain, route.plan.Dest, input)
		if e != nil {
			return outcome{err: e}
		}
		p, err = importer.FromFile(filepath.Base(input), source, title, raw)
	}
	if err != nil {
		return outcome{err: err}
	}
	draft := item.Draft{Kind: item.Source, Status: item.Provisional, Origin: p.Origin, Title: p.Title, Source: p.Source, SHA256: p.SHA256}
	var original *copiedOriginal
	if p.Origin != item.OriginLink {
		digest, err := hex.DecodeString(p.SHA256)
		if err != nil || len(digest) != sha256.Size {
			return outcome{err: ioFailure("could not prepare the original digest")}
		}
		original = &copiedOriginal{Digest: [sha256.Size]byte(digest), Bytes: p.Original}
		draft.Fetched = wc.Now
	}
	draft, err = normalizedDraft(wc, ctx, fields, draft)
	if err != nil {
		return outcome{err: err}
	}
	r, err := publishNew(ctx, wc, newItemInput{route: route, draft: draft, body: []byte(p.Body), original: original})
	return writeOutcome(in.json, r, err)
}
