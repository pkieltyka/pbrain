package bge

// The CLI does not enter goinfer's resident decoder, vision, or KV snapshot
// paths. These reviewed source sites are accepted once each on auditedPin.
var dormantRuntimeSites = map[runtimeSite]string{
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_DELTANET_TIMING"`}:                     "decoder first-use settings",
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_DECODE_TIMING"`}:                       "decoder first-use settings",
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_FAKEQUANT"`}:                           "decoder first-use settings",
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_FAKEQUANT_ACT"`}:                       "decoder first-use settings",
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_FAKEQUANT_EXPERTS"`}:                   "decoder first-use settings",
	{decoderPkg + "/env_snapshot.go", "readSettings", "os.Getenv", `"GOINFER_W4A8_SPLITHALF"`}:                      "decoder first-use settings",
	{decoderPkg + "/cpu_gateup_fused.go", "envBoolDefault", "os.Getenv", "name"}:                                    "decoder first-use boolean settings",
	{cudaModule + "/graphs_safe.go", "(*cudaResident).graphsTenancySafe", "os.Getenv", `"CUDA_MPS_PIPE_DIRECTORY"`}: "resident decoder graph tenancy",
	{cudaModule + "/graphs_safe.go", "(*cudaResident).admitGraphs", "os.Getenv", `"GOINFER_CUDA_GRAPHS_UNSAFE"`}:    "resident decoder graph admission",
	{cudaModule + "/resident_build.go", "(*buildState).newResident", "os.Getenv", `"GOINFER_G4_CAPTURE"`}:           "resident decoder capture",
	{cudaModule + "/resident_build.go", "(*buildState).newResident", "os.Getenv", `"GOINFER_MOE_CACHE_PROF"`}:       "resident decoder profiling",
	{cudaModule + "/resident_build.go", "(*buildState).selectFusion", "os.Getenv", `"GOINFER_CUDA_GRAPHS"`}:         "resident decoder replay",
	{cudaModule + "/resident_build.go", "(*buildState).selectFusion", "os.Getenv", `"GOINFER_CUDA_GRAPHS_SYNC"`}:    "resident decoder replay debugging",
	{cudaModule + "/resident_build.go", "(*buildState).selectFusion", "os.Getenv", `"GOINFER_CUDA_GRAPHS_ONLY"`}:    "resident decoder replay debugging",
	{cudaModule + "/resident.go", "(*cudaResident).lookupKnob", "os.LookupEnv", "name"}:                             "resident decoder without a model knob snapshot",
	{cudaModule + "/vision_encoder.go", "NewVisionEncoder (closure)", "os.Getenv", `"GOINFER_CUDA_VISION_ATTN"`}:    "vision encoder constructor",
	{decoderPkg + "/assets.go", "modelsRoot", "os.Getenv", `"GOINFER_MODELS"`}:                                      "decoder asset-test harness",
	{decoderPkg + "/assets.go", "lookupAsset", "os.Getenv", "env"}:                                                  "decoder asset-test harness",
	{decoderPkg + "/giwverify.go", "giwVerified", "os.Getenv", "giwVerifyEnv"}:                                      "decoder model verification",
	{decoderPkg + "/weights.go", "loadWeights", "os.Getenv", `"GOINFER_P13_OFF"`}:                                   "decoder weight loading",
	{decoderPkg + "/knobs.go", "snapshotKnobs", "os.LookupEnv", "n"}:                                                "decoder option snapshot",
	{decoderPkg + "/knobs.go", "(*knobSet).lookup", "os.LookupEnv", "name"}:                                         "decoder options without a snapshot",
	{decoderPkg + "/knobs.go", "loadKnob", "os.Getenv", "name"}:                                                     "decoder load option fallback",
	{decoderPkg + "/normnoise.go", "applyNormULPNoiseDiag", "os.Getenv", `"GOINFER_NORM_ULP_NOISE"`}:                "decoder residency diagnostics",
	{decoderPkg + "/assets.go", "assetRoot", "os.Getwd", ""}:                                                        "decoder asset-test harness",
	{decoderPkg + "/assets.go", "modelsRoot", "os.UserHomeDir", ""}:                                                 "decoder asset-test harness",
	{decoderPkg + "/selftest_gpu.go", "probeFixture", "os.MkdirTemp", ""}:                                           "decoder GPU self-test fixture",
	{cudaModule + "/pack_stream.go", "pageableStackSources (closure)", "os.CreateTemp", ""}:                         "decoder expert-weight mapping after checking an explicit directory",
}

// The logging functions use the command's redirected os.Stderr at call
// time. Only dormant KV snapshot code uses the original saved stream.
var allowedRuntimeOutput = map[runtimeSite]string{
	{goinferModule + "/internal/runtime/logs/logs.go", "logf", "os.Stderr", ""}:      "call-time stderr after the redirect",
	{goinferModule + "/internal/runtime/logs/logs.go", "writeText", "os.Stderr", ""}: "call-time stderr after the redirect",
	{decoderPkg + "/kvsnap.go", "stderrKV", "os.Stderr", ""}:                         "decoder KV snapshots only",
}

// goinfer #70 moved the v0.6.4 startup reads to first decoder use.
var knownStartupViolations = map[runtimeSite]struct{}{}
