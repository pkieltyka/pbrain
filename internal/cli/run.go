package cli

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/skill"
)

// version can be supplied through -ldflags -X in a release build.
var version string

func binaryVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// Run executes with the feature set selected by the release.
func Run(ctx context.Context, env Env) int { return RunWith(ctx, env, Release) }

// RunWith executes one invocation with explicit feature availability.
func RunWith(ctx context.Context, env Env, features Features) (exit int) {
	jsonMode := false
	defer func() {
		if recover() != nil {
			emitError(env.Stderr, jsonMode, ioFailure("unexpected command failure"))
			exit = 1
		}
	}()

	in, err := parseOptions(env.Args, features)
	jsonMode = in.json
	if err != nil {
		return emitError(env.Stderr, jsonMode, err)
	}
	if err := in.captureTime(&env); err != nil {
		return emitError(env.Stderr, jsonMode, err)
	}
	return finish(env, &in, execute(ctx, env, &in, features))
}

// captureTime takes the invocation snapshot that discovery and all handlers share.
func (in *invocation) captureTime(env *Env) error {
	if env.Now != nil {
		in.now = env.Now()
	}
	now := in.now
	env.Now = func() time.Time { return now }
	in.zone = now.Location()
	if in.boolean("help") || in.boolean("version") {
		return nil
	}
	if env.Getenv == nil {
		env.Getenv = func(string) string { return "" }
	}
	if zone := env.Getenv("TZ"); zone != "" {
		name := strings.TrimPrefix(zone, ":")
		var loc *time.Location
		if name != "Local" && name != "" {
			loc, _ = time.LoadLocation(name)
		}
		if loc != nil {
			in.zone = loc
		} else if needsCivilDay(in.string("since")) || needsCivilDay(in.string("until")) {
			return usage("TZ must name a time zone for a local calendar date")
		}
	}
	if in.has("since") && in.string("since") == "" || in.has("until") && in.string("until") == "" {
		return usage("--since and --until require a date, instant, today, yesterday or span")
	}
	w, err := query.ParseWindow(in.string("since"), in.string("until"), in.now, in.zone)
	in.filter.Window = w
	return err
}

func needsCivilDay(value string) bool {
	if value == "today" || value == "yesterday" {
		return true
	}
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func execute(ctx context.Context, env Env, in *invocation, features Features) outcome {
	if in.boolean("version") {
		value := binaryVersion()
		return textOrJSON(in.json, value+"\n", versionOutput{Version: value})
	}
	if in.boolean("help") {
		value := helpText(in.cmd)
		return textOrJSON(in.json, value, helpOutput{Help: value})
	}
	if in.cmd.name == "skill" {
		return runSkill(in)
	}

	bEnv := brain.Env{Dir: env.Dir, Getenv: env.Getenv, Now: env.Now, Git: env.Git, Files: env.Files}
	open := brain.OpenOptions{Brain: in.string("brain")}
	if in.cmd.name == "init" {
		return runInit(ctx, bEnv, brain.InitOptions{OpenOptions: open, Path: in.string("path")}, in.json)
	}
	if env.Files == nil {
		return outcome{err: ioFailure("file access is unavailable")}
	}
	b, err := brain.Open(ctx, bEnv, open)
	if err != nil {
		return outcome{err: err}
	}
	defer b.Close()
	// Audience edits and listings address the settings file, even if the
	// configured personal section cannot be opened.
	if in.cmd.name != "audience" {
		if err := b.RequireView(in.view); err != nil {
			return outcome{err: err}
		}
	}
	in.useConfig(b.Config)
	return runCommand(ctx, env, in, b, features)
}

func runSkill(in *invocation) outcome {
	var document string
	switch {
	case len(in.operands) == 0:
		document = skill.Core()
	case in.operands[0] == "list":
		topics := skill.Topics()
		return textOrJSON(in.json, strings.Join(topics, "\n")+"\n", struct {
			Topics []string `json:"topics"`
		}{topics})
	default:
		var ok bool
		document, ok = skill.Topic(in.operands[0])
		if !ok {
			return outcome{err: usage("unknown skill topic; use pbrain skill list")}
		}
	}
	return textOrJSON(in.json, document, struct {
		Text string `json:"text"`
	}{document})
}

// runCommand is the dispatch point for handlers delivered after the CLI core.
func runCommand(ctx context.Context, env Env, in *invocation, b *brain.Brain, features Features) outcome {
	switch in.cmd.name {
	case "audience":
		return runAudienceCommand(ctx, env, in, b)
	case "search":
		return runSearch(ctx, env, in, b, features)
	case "get":
		return runGet(ctx, env, in, b)
	case "add":
		return runAddCommand(ctx, in, newWriteContext(env, in, b))
	case "verify":
		actor := in.string("by")
		if in.has("by") && actor == "" {
			return outcome{err: invalidWrite("verified: must not be empty")}
		}
		r, err := runVerify(ctx, newWriteContext(env, in, b), verifyRequest{ID: in.operands[0], By: actor})
		return writeOutcome(in.json, r, err)
	case "delete":
		r, err := runDelete(ctx, newWriteContext(env, in, b), deleteRequest{ID: in.operands[0], Yes: in.boolean("yes")})
		return deleteOutcome(in.json, r, err)
	default:
		return notImplemented()
	}
}

func notImplemented() outcome { return outcome{err: ioFailure("command is not implemented yet")} }

func (in *invocation) useConfig(config brain.Config) {
	limit, maxChars := in.page.limit, in.page.maxChars
	switch in.cmd.name {
	case "search":
		limit, maxChars = config.Search.Limit, config.Search.MaxChars
	case "list":
		limit, maxChars = config.List.Limit, config.List.MaxChars
	case "get":
		if !in.boolean("history") && !in.boolean("outline") {
			maxChars = config.Get.MaxChars
		}
	case "prime":
		maxChars = config.Prime.MaxChars
	}
	if !in.has("limit") && limit > 0 {
		in.page.limit = limit
	}
	if !in.has("max-chars") && maxChars > 0 {
		in.page.maxChars = maxChars
	}
}

type initOutput struct {
	Path string `json:"path"`
	Hint string `json:"hint,omitempty"`
}

func runInit(ctx context.Context, env brain.Env, opt brain.InitOptions, jsonMode bool) outcome {
	if env.Files == nil {
		return outcome{err: ioFailure("file access is unavailable")}
	}
	init, err := brain.Init(ctx, env, opt)
	if err != nil {
		return outcome{err: err}
	}
	result := initOutput{Path: render.OneLine(init.Path, 0)}
	output := "initialized " + result.Path + "\n"
	if init.NeedsSelector {
		result.Hint = "Outside this folder, use --brain or PBRAIN_DIR."
		output += result.Hint + "\n"
	}
	return textOrJSON(jsonMode, output, result)
}

func ioFailure(message string) *errs.Error {
	return &errs.Error{Code: "io", Message: message, Exit: 1}
}
