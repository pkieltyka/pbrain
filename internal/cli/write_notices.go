package cli

import (
	"fmt"
	"strings"
	"time"

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
		fields := []secretscan.Field{
			{Name: "kind", Value: d.Kind.String()},
			{Name: "title", Value: d.Title},
			{Name: "created", Value: d.Created.UTC().Format(time.RFC3339)},
			{Name: "author", Value: d.Author},
			{Name: "source", Value: d.Source},
			{Name: "status", Value: d.Status.String()},
			{Name: "verified", Value: d.Verified},
			{Name: "decided", Value: d.Decided},
			{Name: "replaces", Value: strings.Join(d.Replaces, "\n")},
			{Name: "relates", Value: strings.Join(d.Relates, "\n")},
			{Name: "tags", Value: strings.Join(d.Tags, "\n")},
			{Name: "stale_after", Value: d.StaleAfter},
			{Name: "origin", Value: d.Origin.String()},
			{Name: "fetched", Value: d.Fetched.UTC().Format(time.RFC3339)},
			{Name: "sha256", Value: d.SHA256},
			{Name: "body", Value: string(body)},
		}
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
