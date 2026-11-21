// Package cli: exported surface from the pbrain specs; bodies are filled in by later phases.
package cli

import (
	"context"
	"io"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/importer/fetch"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type Env struct {
	Args           []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	Dir            string
	Now            func() time.Time
	Rand           io.Reader
	Git            gitx.Runner
	Net            fetch.Net
	Files          sys.Files
	Models         embed.Registry
}

type Features struct{ BM25 bool }

var Release = Features{}

func Run(ctx context.Context, env Env) (exit int)                 { panic("not implemented") }
func RunWith(ctx context.Context, env Env, f Features) (exit int) { panic("not implemented") }
