package main

import (
	"log"
	"os"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/gemma2"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: phase28-pack SOURCE_DIR OUTPUT_DIR")
	}
	files := sys.Native()
	source, err := files.OpenRoot(os.Args[1], false)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	pack, err := files.OpenRoot(os.Args[2], false)
	if err != nil {
		log.Fatal(err)
	}
	defer pack.Close()
	entry := gemma2.Entry()
	if err := embed.Pack(files, embed.NewRegistry(entry), entry.Model, source, pack); err != nil {
		log.Fatal(err)
	}
}
