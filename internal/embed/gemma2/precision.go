package gemma2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/pkieltyka/goinfer/embeddinggemma2"
	"github.com/pkieltyka/pbrain/internal/embed"
)

const goinferRevision = "897943c7a6bba158a74eaa3e2c43b1d11b24c445"

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
		return selectFP16(listing, selector, linked, cudaCapability)
	}
}

func selectFP16(listing embed.Listing, selector string, linked bool, capability func(embed.DeviceInfo) (int, int, bool)) (embed.DeviceInfo, string) {
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
	major, minor, ok := capability(device)
	if !ok {
		return reason("could not verify compute capability")
	}
	if major < 8 {
		return reason(fmt.Sprintf("supports compute capability %d.%d; fp16 requires 8.0", major, minor))
	}
	return device, ""
}

func sameCUDAUUID(id, uuid string) bool {
	token, ok := strings.CutPrefix(id, "cuda:")
	return ok && token != "" && strings.EqualFold(token, uuid)
}
