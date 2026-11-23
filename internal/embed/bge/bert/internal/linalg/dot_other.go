//go:build !amd64 && !arm64

// Adapted from aikit linalg/dot_other.go. See LICENSE and NOTICE in the bert directory.
package linalg

func dotNEON4x4(a, b0, b1, b2, b3 *float32, n4 int, sums *[16]float32) {
	dot4x4Generic(a, b0, b1, b2, b3, n4, sums)
}

func dotNEON8x4(a, b0, b1, b2, b3, b4, b5, b6, b7 *float32, n4 int, sums *[32]float32) {
	dot8x4Generic(a, b0, b1, b2, b3, b4, b5, b6, b7, n4, sums)
}
