package embed

import (
	"crypto/sha256"
	"encoding/binary"
)

// VerityDigest computes the Linux fs-verity v1 file digest with SHA-256,
// 4,096-byte blocks and no salt. This is used only to qualify release pins.
func VerityDigest(data []byte) [32]byte {
	const blockSize = 4096
	var root [32]byte
	if len(data) != 0 {
		level := data
		for len(level) > blockSize {
			next := make([]byte, 0, ((len(level)-1)/blockSize+1)*sha256.Size)
			for len(level) > 0 {
				n := min(len(level), blockSize)
				sum := verityBlock(level[:n])
				next = append(next, sum[:]...)
				level = level[n:]
			}
			level = next
		}
		root = verityBlock(level)
	}
	// The descriptor includes file size to disambiguate zero-padded tails
	// and data whose bytes happen to equal an internal tree block.
	descriptor := [256]byte{1, 1, 12}
	binary.LittleEndian.PutUint64(descriptor[8:], uint64(len(data)))
	copy(descriptor[16:], root[:])
	return sha256.Sum256(descriptor[:])
}

func verityBlock(data []byte) [32]byte {
	if len(data) == 4096 {
		return sha256.Sum256(data)
	}
	var padded [4096]byte
	copy(padded[:], data)
	return sha256.Sum256(padded[:])
}
