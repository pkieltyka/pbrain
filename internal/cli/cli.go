// Package cli runs one pbrain command with its supplied process inputs.
package cli

import (
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
