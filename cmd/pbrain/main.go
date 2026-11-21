// Command pbrain keeps a project's knowledge in Markdown files with front matter.
package main

import (
	"context"
	"os"

	"github.com/pkieltyka/pbrain/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), cli.Env{}))
}
