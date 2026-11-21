package cli

import (
	"fmt"
	"time"

	"github.com/pkieltyka/pbrain/internal/check"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/secretscan"
	"github.com/pkieltyka/pbrain/internal/store"
)

func warnSecrets(wc writeContext, fields []secretscan.Field) {
	for _, m := range secretscan.Scan(fields) {
		wc.notify("secret_suspected", fmt.Sprintf("%s line %d: %s", m.Field, m.Line, m.Pattern))
	}
}

func warnNewItem(wc writeContext, plan store.WritePlan, d *item.Draft, body []byte) {
	if plan.Dest == item.Shared {
		fields := check.Stored{
			Kind: d.Kind.String(), Title: d.Title, Created: d.Created.UTC().Format(time.RFC3339),
			Author: d.Author, Source: d.Source, Status: d.Status.String(), Verified: d.Verified, Decided: d.Decided,
			Replaces: d.Replaces, Relates: d.Relates, Tags: d.Tags, StaleAfter: d.StaleAfter,
			Origin: d.Origin.String(), Fetched: d.Fetched.UTC().Format(time.RFC3339), SHA256: d.SHA256,
			Body: string(body),
		}.Fields()
		warnSecrets(wc, fields)
	}
	if wc.Brain.Config.HasTags {
		for _, tag := range d.Tags {
			if _, declared := wc.Brain.Config.Tags[tag]; declared {
				continue
			}
			message := "tags: undeclared tag"
			if !secretscan.Sensitive(tag) {
				message += " " + tag
			}
			wc.notify("undeclared_tag", message)
		}
	}
}
