package gemma2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const goinferRevision = "c0a5159bbff1c2fe5c8624e1399aa7438c820677"

// Both release rows use the same 24-layer text tower. Match the ordinary
// runtime's upload floor when choosing the first candidate without loading it.
const cudaWeightFloor = 3 * 4 * 24 * 512 * 2048

func fp16Fingerprint(base, revision string) string {
	sum := sha256.Sum256([]byte("pbrain-vector-profile-v1\n" + base + "\n" + embed.CUDAFP16Selector + "\n" + revision))
	return hex.EncodeToString(sum[:])
}

func fp16Resolver() func(embed.Listing, string) (embed.DeviceInfo, string) {
	if !cudaCompiled {
		return nil
	}
	return func(listing embed.Listing, selector string) (embed.DeviceInfo, string) {
		linked := slices.Contains(embeddinggemma2.DeviceSelectors("cuda"), embed.CUDAFP16Selector)
		return selectFP16(listing, selector, linked)
	}
}

func selectFP16(listing embed.Listing, selector string, linked bool) (embed.DeviceInfo, string) {
	var device embed.DeviceInfo
	if selector == "" || selector == "auto" {
		for _, d := range listing.Sorted() {
			if d.Memory != "live" || d.FreeBytes >= cudaWeightFloor {
				device = d
				break
			}
		}
	} else {
		device, _ = listing.Match(selector)
	}
	if device.Backend != "cuda" {
		return embed.DeviceInfo{}, ""
	}
	reason := func(why string) (embed.DeviceInfo, string) {
		return embed.DeviceInfo{}, device.ID + " " + why + "; using fp32 CUDA"
	}
	if !linked {
		return reason("has no linked fp16 CUDA kernel")
	}
	if slices.Contains(device.RunnableSelectors, embed.CUDAFP16Selector) {
		return device, ""
	}
	cc := device.ComputeCapability
	if cc.Major == 0 {
		return reason("could not verify compute capability")
	}
	if cc.Major < 8 {
		return reason(fmt.Sprintf("supports compute capability %d.%d; fp16 requires 8.0", cc.Major, cc.Minor))
	}
	return reason("does not report a runnable fp16 CUDA selector")
}
