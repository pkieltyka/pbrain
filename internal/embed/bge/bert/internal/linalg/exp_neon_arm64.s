// NEON f32 exp kernel for the S-06 step-2 numeric contract.
//
// This implements expF32Contract (exp_contract.go) instruction for instruction:
// same algorithm, same constants, same operation ORDER, and every multiply-add a
// true fused FMLA/FMLS. The Go oracle reaches the same correctly-rounded f32 FMA
// through f64 (see fma32 for why that is exact), so the two agree BIT FOR BIT and
// TestExpF32NEON_bitIdenticalToContract asserts raw Float32bits equality rather
// than a tolerance.
//
// Operand order was verified empirically before this was written, because a
// reversed VFSUB or VFMLS is silent: VFSUB Vm,Vn,Vd is Vd = Vn - Vm, VFMLA Vc,Vb,Vd
// is Vd += Vb*Vc, VFMLS Vc,Vb,Vd is Vd -= Vb*Vc, VSSHR is arithmetic.
//
// SCALING IS TWO-STEP AND THAT IS DELIBERATE. The scalar builds 2^k in one shift
// and needs a branch for k=128 (where the exponent field would encode Inf and
// poison a finite answer). Here k is split as k1 = k>>1, k2 = k-k1, and the result
// is p * 2^k1 * 2^k2. For the caller-guaranteed range x in [-87.34, 88.72] we have
// k in [-126,128], so k1,k2 in [-63,64] and both exponent fields are always valid
// normals -- no branch, no Inf encoding. It is EXACT and therefore bit-identical to
// the single-step form, because multiplying by a power of two rounds nothing when
// the intermediate stays normal, and p is in [0.7,1.42] so it does.
//
// Underflow still needs the flush the scalar performs (e<=0 returns 0 rather than a
// subnormal), done branchlessly with a compare-and-mask.

//go:build arm64

#include "textflag.h"

DATA cLog2e<>+0(SB)/4, $0x3fb8aa3b
DATA cLog2e<>+4(SB)/4, $0x3fb8aa3b
DATA cLog2e<>+8(SB)/4, $0x3fb8aa3b
DATA cLog2e<>+12(SB)/4, $0x3fb8aa3b
GLOBL cLog2e<>(SB), RODATA|NOPTR, $16
DATA cMagic<>+0(SB)/4, $0x4b400000
DATA cMagic<>+4(SB)/4, $0x4b400000
DATA cMagic<>+8(SB)/4, $0x4b400000
DATA cMagic<>+12(SB)/4, $0x4b400000
GLOBL cMagic<>(SB), RODATA|NOPTR, $16
DATA cLn2Hi<>+0(SB)/4, $0x3f317200
DATA cLn2Hi<>+4(SB)/4, $0x3f317200
DATA cLn2Hi<>+8(SB)/4, $0x3f317200
DATA cLn2Hi<>+12(SB)/4, $0x3f317200
GLOBL cLn2Hi<>(SB), RODATA|NOPTR, $16
DATA cLn2Lo<>+0(SB)/4, $0x35bfbe8e
DATA cLn2Lo<>+4(SB)/4, $0x35bfbe8e
DATA cLn2Lo<>+8(SB)/4, $0x35bfbe8e
DATA cLn2Lo<>+12(SB)/4, $0x35bfbe8e
GLOBL cLn2Lo<>(SB), RODATA|NOPTR, $16
DATA cP0<>+0(SB)/4, $0x39506967
DATA cP0<>+4(SB)/4, $0x39506967
DATA cP0<>+8(SB)/4, $0x39506967
DATA cP0<>+12(SB)/4, $0x39506967
GLOBL cP0<>(SB), RODATA|NOPTR, $16
DATA cP1<>+0(SB)/4, $0x3ab743ce
DATA cP1<>+4(SB)/4, $0x3ab743ce
DATA cP1<>+8(SB)/4, $0x3ab743ce
DATA cP1<>+12(SB)/4, $0x3ab743ce
GLOBL cP1<>(SB), RODATA|NOPTR, $16
DATA cP2<>+0(SB)/4, $0x3c088908
DATA cP2<>+4(SB)/4, $0x3c088908
DATA cP2<>+8(SB)/4, $0x3c088908
DATA cP2<>+12(SB)/4, $0x3c088908
GLOBL cP2<>(SB), RODATA|NOPTR, $16
DATA cP3<>+0(SB)/4, $0x3d2aa9c1
DATA cP3<>+4(SB)/4, $0x3d2aa9c1
DATA cP3<>+8(SB)/4, $0x3d2aa9c1
DATA cP3<>+12(SB)/4, $0x3d2aa9c1
GLOBL cP3<>(SB), RODATA|NOPTR, $16
DATA cP4<>+0(SB)/4, $0x3e2aaaaa
DATA cP4<>+4(SB)/4, $0x3e2aaaaa
DATA cP4<>+8(SB)/4, $0x3e2aaaaa
DATA cP4<>+12(SB)/4, $0x3e2aaaaa
GLOBL cP4<>(SB), RODATA|NOPTR, $16
DATA cP5<>+0(SB)/4, $0x3f000000
DATA cP5<>+4(SB)/4, $0x3f000000
DATA cP5<>+8(SB)/4, $0x3f000000
DATA cP5<>+12(SB)/4, $0x3f000000
GLOBL cP5<>(SB), RODATA|NOPTR, $16
DATA cOne<>+0(SB)/4, $0x3f800000
DATA cOne<>+4(SB)/4, $0x3f800000
DATA cOne<>+8(SB)/4, $0x3f800000
DATA cOne<>+12(SB)/4, $0x3f800000
GLOBL cOne<>(SB), RODATA|NOPTR, $16

DATA cClamp<>+0(SB)/4, $0xc2d00000
DATA cClamp<>+4(SB)/4, $0xc2d00000
DATA cClamp<>+8(SB)/4, $0xc2d00000
DATA cClamp<>+12(SB)/4, $0xc2d00000
GLOBL cClamp<>(SB), RODATA|NOPTR, $16


DATA cSignMask<>+0(SB)/4, $0x80000000
DATA cSignMask<>+4(SB)/4, $0x80000000
DATA cSignMask<>+8(SB)/4, $0x80000000
DATA cSignMask<>+12(SB)/4, $0x80000000
GLOBL cSignMask<>(SB), RODATA|NOPTR, $16

DATA cErfSeries<>+0(SB)/4, $0x32617183
DATA cErfSeries<>+4(SB)/4, $0x32617183
DATA cErfSeries<>+8(SB)/4, $0x32617183
DATA cErfSeries<>+12(SB)/4, $0x32617183
DATA cErfSeries<>+16(SB)/4, $0xb41bbbe3
DATA cErfSeries<>+20(SB)/4, $0xb41bbbe3
DATA cErfSeries<>+24(SB)/4, $0xb41bbbe3
DATA cErfSeries<>+28(SB)/4, $0xb41bbbe3
DATA cErfSeries<>+32(SB)/4, $0x35c3d001
DATA cErfSeries<>+36(SB)/4, $0x35c3d001
DATA cErfSeries<>+40(SB)/4, $0x35c3d001
DATA cErfSeries<>+44(SB)/4, $0x35c3d001
DATA cErfSeries<>+48(SB)/4, $0xb75debbd
DATA cErfSeries<>+52(SB)/4, $0xb75debbd
DATA cErfSeries<>+56(SB)/4, $0xb75debbd
DATA cErfSeries<>+60(SB)/4, $0xb75debbd
DATA cErfSeries<>+64(SB)/4, $0x38e00e01
DATA cErfSeries<>+68(SB)/4, $0x38e00e01
DATA cErfSeries<>+72(SB)/4, $0x38e00e01
DATA cErfSeries<>+76(SB)/4, $0x38e00e01
DATA cErfSeries<>+80(SB)/4, $0xba46980c
DATA cErfSeries<>+84(SB)/4, $0xba46980c
DATA cErfSeries<>+88(SB)/4, $0xba46980c
DATA cErfSeries<>+92(SB)/4, $0xba46980c
DATA cErfSeries<>+96(SB)/4, $0x3b97b426
DATA cErfSeries<>+100(SB)/4, $0x3b97b426
DATA cErfSeries<>+104(SB)/4, $0x3b97b426
DATA cErfSeries<>+108(SB)/4, $0x3b97b426
DATA cErfSeries<>+112(SB)/4, $0xbcc30c31
DATA cErfSeries<>+116(SB)/4, $0xbcc30c31
DATA cErfSeries<>+120(SB)/4, $0xbcc30c31
DATA cErfSeries<>+124(SB)/4, $0xbcc30c31
DATA cErfSeries<>+128(SB)/4, $0x3dcccccd
DATA cErfSeries<>+132(SB)/4, $0x3dcccccd
DATA cErfSeries<>+136(SB)/4, $0x3dcccccd
DATA cErfSeries<>+140(SB)/4, $0x3dcccccd
DATA cErfSeries<>+144(SB)/4, $0xbeaaaaab
DATA cErfSeries<>+148(SB)/4, $0xbeaaaaab
DATA cErfSeries<>+152(SB)/4, $0xbeaaaaab
DATA cErfSeries<>+156(SB)/4, $0xbeaaaaab
DATA cErfSeries<>+160(SB)/4, $0x3f800000
DATA cErfSeries<>+164(SB)/4, $0x3f800000
DATA cErfSeries<>+168(SB)/4, $0x3f800000
DATA cErfSeries<>+172(SB)/4, $0x3f800000
GLOBL cErfSeries<>(SB), RODATA|NOPTR, $176
DATA cErfAS<>+0(SB)/4, $0x3f87dc22
DATA cErfAS<>+4(SB)/4, $0x3f87dc22
DATA cErfAS<>+8(SB)/4, $0x3f87dc22
DATA cErfAS<>+12(SB)/4, $0x3f87dc22
DATA cErfAS<>+16(SB)/4, $0xbfba00e3
DATA cErfAS<>+20(SB)/4, $0xbfba00e3
DATA cErfAS<>+24(SB)/4, $0xbfba00e3
DATA cErfAS<>+28(SB)/4, $0xbfba00e3
DATA cErfAS<>+32(SB)/4, $0x3fb5f0e3
DATA cErfAS<>+36(SB)/4, $0x3fb5f0e3
DATA cErfAS<>+40(SB)/4, $0x3fb5f0e3
DATA cErfAS<>+44(SB)/4, $0x3fb5f0e3
DATA cErfAS<>+48(SB)/4, $0xbe91a98e
DATA cErfAS<>+52(SB)/4, $0xbe91a98e
DATA cErfAS<>+56(SB)/4, $0xbe91a98e
DATA cErfAS<>+60(SB)/4, $0xbe91a98e
DATA cErfAS<>+64(SB)/4, $0x3e827906
DATA cErfAS<>+68(SB)/4, $0x3e827906
DATA cErfAS<>+72(SB)/4, $0x3e827906
DATA cErfAS<>+76(SB)/4, $0x3e827906
GLOBL cErfAS<>(SB), RODATA|NOPTR, $80
DATA cTwoOverSqrtPi<>+0(SB)/4, $0x3f906ebb
DATA cTwoOverSqrtPi<>+4(SB)/4, $0x3f906ebb
DATA cTwoOverSqrtPi<>+8(SB)/4, $0x3f906ebb
DATA cTwoOverSqrtPi<>+12(SB)/4, $0x3f906ebb
GLOBL cTwoOverSqrtPi<>(SB), RODATA|NOPTR, $16
DATA cASk<>+0(SB)/4, $0x3ea7ba05
DATA cASk<>+4(SB)/4, $0x3ea7ba05
DATA cASk<>+8(SB)/4, $0x3ea7ba05
DATA cASk<>+12(SB)/4, $0x3ea7ba05
GLOBL cASk<>(SB), RODATA|NOPTR, $16

// func expF32ContractNEON(dst, src *float32, n int)
// n must be a multiple of 4; the Go caller handles the tail. Inputs must be finite
// and within [expUnderflowF32, expOverflowF32] -- the caller guards, exactly as
// ExpF32 guards expF32Core.

TEXT ·expBiasSumF32NEON(SB), NOSPLIT, $0-40
	MOVD dst+0(FP), R0
	MOVD src+8(FP), R1
	MOVD n+16(FP), R2
	FMOVS bias+24(FP), F29
	MOVD partials+32(FP), R5
	VDUP V29.S[0], V29.S4
	MOVD $cLog2e<>(SB), R3
	VLD1 (R3), [V16.S4]
	MOVD $cMagic<>(SB), R3
	VLD1 (R3), [V17.S4]
	MOVD $cLn2Hi<>(SB), R3
	VLD1 (R3), [V18.S4]
	MOVD $cLn2Lo<>(SB), R3
	VLD1 (R3), [V19.S4]
	MOVD $cP0<>(SB), R3
	VLD1 (R3), [V20.S4]
	MOVD $cP1<>(SB), R3
	VLD1 (R3), [V21.S4]
	MOVD $cP2<>(SB), R3
	VLD1 (R3), [V22.S4]
	MOVD $cP3<>(SB), R3
	VLD1 (R3), [V23.S4]
	MOVD $cP4<>(SB), R3
	VLD1 (R3), [V24.S4]
	MOVD $cP5<>(SB), R3
	VLD1 (R3), [V25.S4]
	MOVD $cOne<>(SB), R3
	VLD1 (R3), [V26.S4]
	MOVD $127, R4
	VDUP R4, V27.S4
	VMOVI $0, V28.B16
	VMOVI $0, V30.B16          // partials[0..1]
	VMOVI $0, V31.B16          // partials[2..3]
	MOVD $cClamp<>(SB), R3
	VLD1 (R3), [V15.S4]        // -104: below the flush threshold, but FINITE
	CBZ R2, sumdone

sumloop:
	VLD1.P 16(R1), [V0.S4]
	VFADD V29.S4, V0.S4, V0.S4   // x + bias  (= x - rowMax)
	// Clamp to a finite value that still flushes. A row containing -Inf makes
	// x+bias = -Inf, and VFCVTZS(-Inf) saturates to INT32_MIN while Go's own
	// float->int conversion of -Inf is implementation-specific -- so the two paths
	// would produce DIFFERENT garbage rather than the same answer. -104 is below
	// expUnderflowF32, so exp of it is 0 either way and nothing observable moves.
	VFMAX V15.S4, V0.S4, V0.S4
	// ---- expF32Contract body, four lanes; V0 in, V4 out ----
	VFMUL V16.S4, V0.S4, V1.S4   // z  = x * log2e
	VFADD V17.S4, V1.S4, V1.S4   // t  = z + magic
	VFSUB V17.S4, V1.S4, V1.S4   // kf = t - magic
	VFCVTZS V1.S4, V2.S4         // k  = int32(kf)
	VMOV V0.B16, V3.B16          // r  = x
	VFMLS V18.S4, V1.S4, V3.S4   // r -= kf*ln2Hi
	VFMLS V19.S4, V1.S4, V3.S4   // r -= kf*ln2Lo
	VMOV V21.B16, V4.B16
	VFMLA V3.S4, V20.S4, V4.S4
	VMOV V22.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4
	VMOV V23.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4
	VMOV V24.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4
	VMOV V25.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4
	VMOV V26.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4    // q = p*r + 1
	VMOV V26.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4    // p = q*r + 1
	VSSHR $1, V2.S4, V6.S4
	VSUB V6.S4, V2.S4, V7.S4
	VADD V27.S4, V6.S4, V6.S4
	VADD V27.S4, V7.S4, V7.S4
	VSHL $23, V6.S4, V6.S4
	VSHL $23, V7.S4, V7.S4
	VFMUL V6.S4, V4.S4, V4.S4
	VFMUL V7.S4, V4.S4, V4.S4
	VADD V27.S4, V2.S4, V8.S4
	VCMGT V28.S4, V8.S4, V9.S4
	VAND V9.B16, V4.B16, V4.B16
	VST1.P [V4.S4], 16(R0)
	// widen and accumulate, lanes 0,1 -> partials[0,1]; lanes 2,3 -> partials[2,3]
	VFCVTL V4.S2, V10.D2
	VFCVTL2 V4.S4, V11.D2
	VFADD V10.D2, V30.D2, V30.D2
	VFADD V11.D2, V31.D2, V31.D2
	SUBS $4, R2, R2
	BNE sumloop
sumdone:
	VST1 [V30.D2, V31.D2], (R5)
	RET

// func siluF32ContractNEON(dst, src *float32, n int)
//
// dst[i] = x / (1 + exp(clamp(-x))), the SwiGLU gate activation, four lanes at a
// time. n must be a multiple of 4.
//
// THE CLAMP IS LOAD-BEARING, not defensive. expF32Contract's overflow branch
// computes uint32(e-1)<<23, which is a valid exponent field only while e <= 255;
// at e >= 256 it overflows into the SIGN bit and returns -0. ExpF32's own guard
// makes that unreachable, but silu feeds it -x, so an unclamped x < -89.4 would
// walk straight into it. Clamping -x to [-104, 88.72283] keeps both paths inside
// the range where they are defined and agree. The cost is only in an extreme
// tail: for x below -88.72 the result is x/(1+3.4e38), a tiny nonzero rather
// than the signed zero SiLUF32 produces from an infinite denominator.

TEXT ·erfF32ContractNEON(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0
	MOVD src+8(FP), R1
	MOVD n+16(FP), R2
	CBZ  R2, erfdone
	MOVD $cLog2e<>(SB), R3
	VLD1 (R3), [V16.S4]
	MOVD $cMagic<>(SB), R3
	VLD1 (R3), [V17.S4]
	MOVD $cLn2Hi<>(SB), R3
	VLD1 (R3), [V18.S4]
	MOVD $cLn2Lo<>(SB), R3
	VLD1 (R3), [V19.S4]
	MOVD $cP0<>(SB), R3
	VLD1 (R3), [V20.S4]
	MOVD $cP1<>(SB), R3
	VLD1 (R3), [V21.S4]
	MOVD $cP2<>(SB), R3
	VLD1 (R3), [V22.S4]
	MOVD $cP3<>(SB), R3
	VLD1 (R3), [V23.S4]
	MOVD $cP4<>(SB), R3
	VLD1 (R3), [V24.S4]
	MOVD $cP5<>(SB), R3
	VLD1 (R3), [V25.S4]
	MOVD $cOne<>(SB), R3
	VLD1 (R3), [V26.S4]
	MOVD $127, R4
	VDUP R4, V27.S4
	VMOVI $0, V28.B16

erfloop:
	VLD1.P 16(R1), [V1.S4]        // x
	MOVD $cSignMask<>(SB), R3
	VLD1 (R3), [V2.S4]
	VAND V2.B16, V1.B16, V11.B16  // signbit
	VFABS V1.S4, V10.S4           // a = |x|

	// ---- Maclaurin branch, a < 1 ----
	VFMUL V10.S4, V10.S4, V1.S4   // z = a*a
	MOVD $cErfSeries<>(SB), R5
	VLD1.P 16(R5), [V2.S4]
	VLD1.P 16(R5), [V3.S4]
	VFMLA V1.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V1.S4, V3.S4, V2.S4
	VLD1.P 16(R5), [V3.S4]
	VFMLA V1.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V1.S4, V3.S4, V2.S4
	VLD1.P 16(R5), [V3.S4]
	VFMLA V1.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V1.S4, V3.S4, V2.S4
	VLD1.P 16(R5), [V3.S4]
	VFMLA V1.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V1.S4, V3.S4, V2.S4
	VLD1.P 16(R5), [V3.S4]
	VFMLA V1.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V1.S4, V3.S4, V2.S4
	MOVD $cTwoOverSqrtPi<>(SB), R3
	VLD1 (R3), [V5.S4]
	VFMUL V10.S4, V5.S4, V6.S4    // (2/sqrt(pi)) * a
	VFMUL V2.S4, V6.S4, V12.S4 // series = that * p

	// ---- A&S tail branch, a >= 1 ----
	MOVD $cASk<>(SB), R3
	VLD1 (R3), [V5.S4]
	VMOV V26.B16, V6.B16
	VFMLA V10.S4, V5.S4, V6.S4    // 1 + 0.3275911*a   (fused, as the contract says)
	VFDIV V6.S4, V26.S4, V13.S4   // t = 1/that
	MOVD $cErfAS<>(SB), R5
	VLD1.P 16(R5), [V2.S4]
	VLD1.P 16(R5), [V3.S4]
	VFMLA V13.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V13.S4, V3.S4, V2.S4
	VLD1.P 16(R5), [V3.S4]
	VFMLA V13.S4, V2.S4, V3.S4
	VLD1.P 16(R5), [V2.S4]
	VFMLA V13.S4, V3.S4, V2.S4
	VMOV V2.B16, V14.B16       // q

	// ---- e^(-a*a), clamped so the exponent build stays in range ----
	VFMUL V10.S4, V10.S4, V0.S4
	VFNEG V0.S4, V0.S4
	MOVD $cClamp<>(SB), R3
	VLD1 (R3), [V2.S4]
	VFMAX V2.S4, V0.S4, V0.S4
	VFMUL V16.S4, V0.S4, V1.S4   // z  = x * log2e
	VFADD V17.S4, V1.S4, V1.S4   // t  = z + magic
	VFSUB V17.S4, V1.S4, V1.S4   // kf = t - magic
	VFCVTZS V1.S4, V2.S4         // k  = int32(kf)
	VMOV V0.B16, V3.B16          // r  = x
	VFMLS V18.S4, V1.S4, V3.S4   // r -= kf*ln2Hi
	VFMLS V19.S4, V1.S4, V3.S4   // r -= kf*ln2Lo
	VMOV V21.B16, V4.B16
	VFMLA V3.S4, V20.S4, V4.S4
	VMOV V22.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4
	VMOV V23.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4
	VMOV V24.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4
	VMOV V25.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4
	VMOV V26.B16, V5.B16
	VFMLA V3.S4, V4.S4, V5.S4    // q = p*r + 1
	VMOV V26.B16, V4.B16
	VFMLA V3.S4, V5.S4, V4.S4    // p = q*r + 1
	VSSHR $1, V2.S4, V6.S4
	VSUB V6.S4, V2.S4, V7.S4
	VADD V27.S4, V6.S4, V6.S4
	VADD V27.S4, V7.S4, V7.S4
	VSHL $23, V6.S4, V6.S4
	VSHL $23, V7.S4, V7.S4
	VFMUL V6.S4, V4.S4, V4.S4
	VFMUL V7.S4, V4.S4, V4.S4
	VADD V27.S4, V2.S4, V8.S4
	VCMGT V28.S4, V8.S4, V9.S4
	VAND V9.B16, V4.B16, V4.B16
	VFMUL V13.S4, V14.S4, V5.S4   // q*t
	VMOV V26.B16, V6.B16
	VFMLS V4.S4, V5.S4, V6.S4     // tail = 1 - (q*t)*e   (one fused op)

	// ---- blend and re-sign ----
	VCMGT V10.S4, V26.S4, V2.S4   // mask = (1 > a)
	VBSL V6.B16, V12.B16, V2.B16  // mask ? series : tail
	VORR V11.B16, V2.B16, V2.B16
	VST1.P [V2.S4], 16(R0)
	SUBS $4, R2, R2
	BNE erfloop
erfdone:
	RET
