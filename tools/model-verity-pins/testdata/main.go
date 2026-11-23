// Print fs-verity release pins from locally verified model files.
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/bge"
	"github.com/pkieltyka/pbrain/internal/embed/gemma2"
	"github.com/pkieltyka/pbrain/internal/sys"
)

func main() {
	name := flag.String("model", "", "release model name")
	dir := flag.String("dir", "", "directory containing the final model files")
	flag.Parse()
	log.SetFlags(0)
	if err := printPins(*name, *dir); err != nil {
		log.Fatal(err)
	}
}

func printPins(name, dir string) error {
	if dir == "" {
		return fmt.Errorf("supply -dir with locally verified release files")
	}
	entries := append([]embed.Entry{bge.Entry(), gemma2.Entry()}, gemma2.GGUFEntries()...)
	model, ok := embed.NewRegistry(entries...).Lookup(name)
	if !ok {
		return fmt.Errorf("unknown release model %q", name)
	}
	path, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	files := sys.Native()
	root, _, err := sys.OpenNamed(files, path, false)
	if err != nil {
		return err
	}
	defer root.Close()
	artifact, err := embed.Verify(files, root, model)
	if err != nil {
		return err
	}
	for i, f := range model.Files {
		fmt.Printf("%s\t%d\t%x\t%x\n", f.Name, f.Size, f.SHA256, embed.VerityDigest(artifact.Files[i]))
	}
	return nil
}
