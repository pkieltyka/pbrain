package embedtest

import (
	"os"
	"testing"
)

// AllowGemma2 is the team switch for real EmbeddingGemma 2 workloads in tests and tools: they run
// only when it is "1". The product is unaffected; users can still select embeddinggemma-2.
const AllowGemma2 = "PBRAIN_ALLOW_GEMMA2"

// SkipGemma2 skips t unless the team switch allows real EmbeddingGemma 2 workloads, whether or not
// a model directory is set.
func SkipGemma2(t testing.TB) {
	t.Helper()
	if os.Getenv(AllowGemma2) != "1" {
		t.Skipf("EmbeddingGemma 2 workloads are off for the team: %s=1 turns them on (owner only)", AllowGemma2)
	}
}
