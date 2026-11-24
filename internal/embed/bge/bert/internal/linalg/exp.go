// Adapted from aikit linalg/exp.go. See LICENSE and NOTICE in the bert directory.
package linalg

const (
	log2eF32 = 1.4426950408889634 // 1/ln2

	// ln2 split so that k*ln2Hi is exact in f32 for the |k| ≤ 150 this sees:
	// ln2Hi has its low mantissa bits zeroed, leaving room for the product.
	ln2HiF32 = 0.693145751953125     // ln2 to 12 fractional bits, exact in f32
	ln2LoF32 = 1.428606820309417e-06 // ln2 - ln2Hi

	// roundMagicF32 = 1.5 * 2^23. Adding it to a float32 in [-2^22, 2^22]
	// forces the mantissa to absorb the fractional bits, i.e. round-to-nearest-
	// even, and subtracting it back leaves the rounded integer. Branch-free, and
	// unlike math.Round it is not a function call — math.Round is NOT an amd64
	// intrinsic (perf-campaign item 26; math.Trunc is).
	roundMagicF32 = 12582912.0
)
