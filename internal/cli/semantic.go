package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/search"
	"github.com/pkieltyka/pbrain/internal/semantic"
	"github.com/pkieltyka/pbrain/internal/store"
	"github.com/pkieltyka/pbrain/internal/sys"
	"github.com/pkieltyka/pbrain/internal/trace"
)

// The semantic commands (cli.md "Semantic commands and scan notices", semantic.md "What cli does"
// and "Output and notice texts").

// cachePath is the per-user cache folder from the environment alone, or "" when there is none.
func cachePath(env Env) string {
	get := env.Getenv
	if get == nil {
		return ""
	}
	if p := get("PBRAIN_CACHE"); p != "" {
		if !filepath.IsAbs(p) {
			p = filepath.Join(env.Dir, p)
		}
		return filepath.Clean(p)
	}
	home := get("HOME")
	if runtime.GOOS == "darwin" {
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Caches", "pbrain")
	}
	if x := get("XDG_CACHE_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "pbrain")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "pbrain")
}

// semanticOptions builds the semantic options of one command. The returned function releases
// the cache and the model_path folder.
func semanticOptions(ctx context.Context, env Env, b *brain.Brain, create bool) (semantic.Options, func(), error) {
	reg := env.Models
	if reg == nil {
		reg = embed.NewRegistry()
	}
	rnd := env.Rand
	if rnd == nil {
		rnd = rand.Reader
	}
	opt := semantic.Options{Files: env.Files, Registry: reg, ModelName: b.Config.Semantic.Model, Rand: rnd}
	opt.CUDAPrecision = b.Config.Semantic.CUDAPrecision
	release := func() {}
	if opt.ModelName == "" {
		return opt, release, nil
	}
	if mp := b.Local.ModelPath; mp != "" {
		if !filepath.IsAbs(mp) {
			mp = filepath.Join(b.Dir, mp)
		}
		d, _, err := sys.OpenNamed(env.Files, mp, false)
		if err != nil {
			return opt, release, &errs.Error{Code: "invalid_config", Message: "model_path in personal.toml cannot be opened", Exit: 1}
		}
		opt.ModelDir = d
		release = func() { d.Close() }
	}
	if path := cachePath(env); path != "" {
		c, err := semantic.OpenCache(ctx, semantic.CacheOptions{Path: path, Forbidden: []string{b.Shared.Dir, b.Personal.Dir},
			Git: env.Git, Files: env.Files}, create)
		switch {
		case errors.Is(err, semantic.ErrCacheExposed):
			opt.CacheErr = err
		case err != nil && create:
			release()
			return opt, func() {}, err
		case err != nil:
			opt.CacheErr = semantic.ErrCacheExposed // a search treats a cache it cannot test as unusable
		default:
			opt.Cache = c
			prev := release
			release = func() { c.Close(); prev() }
		}
	}
	return opt, release, nil
}

// sectionDirs are the view's sections: shared, and with --personal the personal one; a default
// personal folder that does not exist has no Dir.
func sectionDirs(in *invocation, b *brain.Brain) ([]semantic.SectionDir, error) {
	shared, err := b.OpenSection(item.Shared)
	if err != nil {
		return nil, err
	}
	dirs := []semantic.SectionDir{{Section: item.Shared, Dir: shared, Root: b.Shared.Dir}}
	if in.view == brain.PersonalView {
		d, err := b.OpenSection(item.Personal)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		dirs = append(dirs, semantic.SectionDir{Section: item.Personal, Dir: d, Root: b.Personal.Dir})
	}
	return dirs, nil
}

// inputs splits one catalog into the inputs of the sections in dirs.
func inputs(dirs []semantic.SectionDir, cat *store.Catalog) []semantic.SectionInput {
	out := make([]semantic.SectionInput, len(dirs))
	for i, d := range dirs {
		out[i] = semantic.SectionInput{SectionDir: d, Body: cat.Body}
		for _, it := range cat.Items() {
			if it.Section == d.Section {
				out[i].Items = append(out[i].Items, it)
			}
		}
	}
	return out
}

// indexHint is "pbrain sync", with --personal in the personal view.
func indexHint(in *invocation) string {
	if in.view == brain.PersonalView {
		return "pbrain sync --personal"
	}
	return "pbrain sync"
}

// semanticNotice is the one notice a search with --semantic prints from Result.Sem.
func semanticNotice(in *invocation, st search.SemanticState) (errs.Notice, bool) {
	x := indexHint(in)
	if !st.Asked {
		return errs.Notice{}, false
	}
	if !st.Ready {
		msg := map[search.NotReady]string{
			search.NoModel:          "semantic search is off: pbrain.toml sets model = \"none\"",
			search.UnsupportedModel: "this pbrain does not support the model pbrain.toml names: pbrain sync lists the supported names",
			search.CacheExposed:     "the cache folder lies in the brain, the personal section or a Git repository: set PBRAIN_CACHE to a folder outside them",
			search.ModelAbsent:      "the model is not on this machine: run " + x + ", or set model_path",
			search.NoIndex:          "no index for this model: run " + x,
			search.ModelFailed:      "the model could not be used: run " + x + " to check it",
		}[st.Reason]
		if msg == "" {
			msg = "the model could not be used: run " + x + " to check it"
		}
		return errs.Notice{Code: "semantic_not_ready", Message: msg}, true
	}
	if st.Missing > 0 {
		verb := " items are"
		if st.Missing == 1 {
			verb = " item is"
		}
		return errs.Notice{Code: "semantic_partial", Message: fmt.Sprintf("%d%s not in the index: run %s", st.Missing, verb, x)}, true
	}
	return errs.Notice{}, false
}

func sectionName(s item.Section) string {
	if s == item.Personal {
		return "personal"
	}
	return "shared"
}

func runIndex(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	if in.boolean("status") {
		return runIndexStatus(ctx, env, in, b)
	}
	runtimeOpt, err := syncRuntimeOptions(env, in)
	if err != nil {
		return outcome{err: err}
	}
	opt, release, err := semanticOptions(ctx, env, b, true)
	defer release()
	if err != nil {
		return outcome{err: err}
	}
	opt.Device, opt.GPUMemory = runtimeOpt.Device, runtimeOpt.MemoryBudget
	opt.MetalSingleton = runtimeOpt.MetalSingleton
	opt.ShareWriterResolution()
	dirs, err := sectionDirs(in, b)
	if err != nil {
		return outcome{err: err}
	}
	notify := func(m embed.Model, bytes int64) {
		emitNotice(env.Stderr, in.json, errs.Notice{Code: "model_download", Message: downloadMessage(m, bytes)})
	}
	shown := map[string]bool{}
	progress := func(p semantic.Progress) {
		if p.PrecisionFallback != "" {
			const code = "cuda_precision_fallback"
			if !shown[code] {
				emitNotice(env.Stderr, in.json, errs.Notice{Code: code, Message: p.PrecisionFallback})
				shown[code] = true
			}
			return
		}
		if p.Run != nil {
			if n, ok := deviceNotice(*p.Run, opt.ModelName); ok && !shown[n.Code] {
				emitNotice(env.Stderr, in.json, n)
				shown[n.Code] = true
			}
			return
		}
		emitNotice(env.Stderr, in.json, errs.Notice{Code: "index_progress", Message: progressMessage(p)})
	}
	skipped := map[string]bool{}
	var lines []string
	var rows []render.Object
	for _, d := range dirs {
		view := brain.SharedView
		if d.Section == item.Personal {
			view = brain.PersonalView
		}
		scan := func(ctx context.Context) (semantic.SectionInput, error) {
			scanSpan := trace.Start(ctx, "brain_scan_read")
			defer scanSpan.End()
			cat, err := store.Scan(ctx, b, view, store.KeepAll)
			if err != nil {
				return semantic.SectionInput{}, err
			}
			for _, s := range cat.Skipped() {
				if s.Section == d.Section && broken(s) {
					skipped[s.Path] = true
				}
			}
			return inputs([]semantic.SectionDir{d}, cat)[0], nil
		}
		res, err := semantic.Build(trace.WithSection(ctx, sectionName(d.Section)), opt, d, scan, fetch.New(env.Net), notify, progress)
		if err != nil {
			return outcome{err: err}
		}
		name := sectionName(res.Section)
		lines = append(lines, fmt.Sprintf("%s: %s indexed, %s removed; %s embedded, %d reused", name,
			counted(res.Items, "item", "items"), counted(res.ItemsRemoved, "item", "items"),
			counted(res.PassagesEmbedded, "passage", "passages"), res.PassagesReused))
		row := render.Object{"section": name, "items": res.Items, "items_removed": res.ItemsRemoved,
			"passages_embedded": res.PassagesEmbedded, "passages_reused": res.PassagesReused}
		if res.Device != "" {
			row["device"] = res.Device
		}
		rows = append(rows, row)
	}
	if n := len(skipped); n > 0 {
		msg := fmt.Sprintf("%d %s skipped: run pbrain check", n, plural(n, "file", "files"))
		if in.view == brain.PersonalView {
			msg += " --personal"
		}
		emitNotice(env.Stderr, in.json, errs.Notice{Code: "skipped_files", Message: msg})
	}
	return textOrJSON(in.json, strings.Join(lines, "\n")+"\n", render.Object{"model": opt.ModelName, "sections": rows})
}

// progressMessage is the index_progress text: "<section>: <Done> of <Total> item[s]", or the wait.
func progressMessage(p semantic.Progress) string {
	if p.Waiting {
		return sectionName(p.Section) + ": waiting for another pbrain command"
	}
	return fmt.Sprintf("%s: %d of %s", sectionName(p.Section), p.Done, counted(p.Total, "item", "items"))
}

// downloadMessage is "downloading model <name>, <n> MB, from <hosts>".
func downloadMessage(m embed.Model, bytes int64) string {
	files := m.Files
	if len(m.Source) > 0 {
		files = m.Source
	}
	var hosts []string
	for _, f := range files {
		if u, err := url.Parse(f.URL); err == nil && u.Host != "" && !contains(hosts, u.Host) {
			hosts = append(hosts, u.Host)
		}
	}
	mb := (bytes + 999_999) / 1_000_000
	return fmt.Sprintf("downloading model %s, %d MB, from %s", m.Name, mb, strings.Join(hosts, ", "))
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func runIndexStatus(ctx context.Context, env Env, in *invocation, b *brain.Brain) outcome {
	opt, release, err := semanticOptions(ctx, env, b, false)
	defer release()
	if err != nil {
		return outcome{err: err}
	}
	dirs, err := sectionDirs(in, b)
	if err != nil {
		return outcome{err: err}
	}
	r := &reader{env: env, in: in, b: b, now: in.now}
	if err := r.scan(ctx, store.KeepAll); err != nil {
		return outcome{err: err}
	}
	res, err := semantic.Status(ctx, opt, inputs(dirs, r.cat))
	if err != nil {
		return outcome{err: err}
	}
	r.skipped()
	var lines []string
	root := render.Object{}
	switch {
	case res.Model == "":
		lines = append(lines, "model: none")
	case !res.Supported:
		lines = append(lines, "model: "+res.Model+", not supported by this pbrain")
	case res.ModelPresent:
		lines = append(lines, "model: "+res.Model+", on this machine")
	default:
		lines = append(lines, "model: "+res.Model+", not on this machine")
	}
	if res.Model != "" {
		root["model"] = res.Model
	}
	if res.ModelPresent {
		root["model_present"] = true
	}
	var rows []render.Object
	for _, s := range res.Sections {
		name := sectionName(s.Section)
		line := fmt.Sprintf("%s: %d of %s indexed", name, s.Indexed, counted(s.Items, "item", "items"))
		row := render.Object{"section": name, "items": s.Items, "indexed": s.Indexed}
		switch s.State {
		case semantic.IndexStale:
			line += " (built for another model)"
			row["stale"] = true
		case semantic.IndexUnreadable:
			line += " (unreadable)"
			row["unreadable"] = true
		}
		lines = append(lines, line)
		rows = append(rows, row)
	}
	root["sections"] = rows
	return textOrJSON(in.json, strings.Join(lines, "\n")+"\n", root)
}
