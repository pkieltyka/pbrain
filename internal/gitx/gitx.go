// Package gitx: exported surface from the pbrain specs; bodies are filled in by later phases.
package gitx

import "context"

type Result struct {
	Stdout []byte
	Status int
}
type Runner interface {
	Run(ctx context.Context, dir string, args []string, stdin []byte) (Result, error)
}
type Path struct {
	Rel       string // native relative path below the Git top
	Directory bool
}

func NewRunner(executable string, environ []string) Runner          { panic("not implemented") }
func Top(ctx context.Context, r Runner, dir string) (string, error) { panic("not implemented") }
func Ignored(ctx context.Context, r Runner, top string, paths []Path) ([]bool, error) {
	panic("not implemented")
}
func Tracked(ctx context.Context, r Runner, top string, path Path) (bool, error) {
	panic("not implemented")
}
func BrainMarkers(ctx context.Context, r Runner, top string) ([]string, error) {
	panic("not implemented")
}
func UserName(ctx context.Context, r Runner, dir string) (name string, found bool, err error) {
	panic("not implemented")
}
