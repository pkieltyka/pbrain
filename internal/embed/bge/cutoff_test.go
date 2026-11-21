package bge

import "testing"

// The release row carries the cut-off the calibration tool chose on the evaluation fixture
// (semantic.md "Calibration"); the fingerprint does not depend on it.
func TestEntryCutoff(t *testing.T) {
	m := Entry().Model
	if m.Cutoff != 0.65 || m.Dim != 384 || len(m.Fingerprint) != 64 {
		t.Fatalf("the release row: %+v", m)
	}
	uncut := makeModel()
	uncut.Cutoff = 0
	if uncut.Fingerprint != m.Fingerprint {
		t.Fatal("the fingerprint depends on the cut-off")
	}
}
