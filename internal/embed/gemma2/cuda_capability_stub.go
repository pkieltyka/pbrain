//go:build !linux || !cuda

package gemma2

import "github.com/pkieltyka/pbrain/internal/embed"

func cudaCapability(embed.DeviceInfo) (int, int, bool) { return 0, 0, false }
