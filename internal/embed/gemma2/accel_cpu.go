//go:build !cuda

package gemma2

import "github.com/pkieltyka/goinfer/embeddinggemma2"

func attachAccelerator(*embeddinggemma2.Encoder) error  { return nil }
func requireAccelerator(*embeddinggemma2.Encoder) error { return nil }
