package cli

import (
	"context"
	"io"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/store"
)

const maxBodyChars = 512000

type addRequest struct {
	itemWrite
	Kind                             item.Kind
	Title, Source, Decided, Verified string
	Body, BodyFile                   *string // nil means absent
}

func runAdd(ctx context.Context, wc writeContext, req addRequest) (writeResult, error) {
	if req.Kind > item.Note {
		return writeResult{}, invalidWrite("kind: add accepts decision, learning, observation or note")
	}
	if req.Body != nil && req.BodyFile != nil {
		return writeResult{}, usage("--body cannot be used with --body-file")
	}
	status := item.Provisional
	switch {
	case req.Kind == item.Decision:
		if req.Decided == "" || req.Verified != "" {
			return writeResult{}, usage("a decision requires --decided and refuses --verified")
		}
		status = item.Verified
	case req.Decided != "":
		return writeResult{}, usage("--decided is only for a decision")
	case req.Verified != "":
		status = item.Verified
	}

	route, err := routeWrite(wc, audience.Request{Explicit: req.Explicit, Source: req.Source, Method: audience.Citation})
	if err != nil {
		return writeResult{}, err
	}
	draft, err := normalizedDraft(wc, ctx, req.itemWrite, item.Draft{
		Kind: req.Kind, Title: req.Title, Source: req.Source, Status: status, Decided: req.Decided, Verified: req.Verified,
	})
	if err != nil {
		return writeResult{}, err
	}
	if route.decision.Matched != "" {
		route.destinationNotice(wc, "")
	}
	body, err := addBody(wc, route.plan.Dest, req)
	if err != nil {
		return writeResult{}, err
	}
	return publishNew(ctx, wc, newItemInput{route: route, draft: draft, body: body})
}

func addBody(wc writeContext, dest item.Section, req addRequest) ([]byte, error) {
	const byteCeiling = maxBodyChars * utf8.UTFMax
	var body []byte
	var err error
	switch {
	case req.Body != nil:
		body = []byte(*req.Body)
	case req.BodyFile != nil:
		body, err = store.OpenInput(wc.Brain, dest, *req.BodyFile)
		if err != nil {
			return nil, err
		}
	case !wc.IsTTY && wc.Stdin != nil:
		body, err = io.ReadAll(io.LimitReader(wc.Stdin, byteCeiling+1))
		if err != nil {
			return nil, ioFailure("could not read the item body from standard input")
		}
	}
	if len(body) > byteCeiling {
		return nil, writeError("too_large", "the body exceeds 512,000 characters")
	}
	if !utf8.Valid(body) {
		return nil, invalidWrite("body: expected UTF-8")
	}
	if utf8.RuneCount(body) > maxBodyChars {
		return nil, writeError("too_large", "the body exceeds 512,000 characters")
	}
	return body, nil
}
