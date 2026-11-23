package bge

// The CLI does not enter goinfer's resident decoder, vision, or KV snapshot
// paths. These reviewed source sites are accepted once each on auditedPin.
var dormantRuntimeSites = map[runtimeSite]string{
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
}

// The logging functions use the command's redirected os.Stderr at call
// time. Only dormant KV snapshot code uses the original saved stream.
var allowedRuntimeOutput = map[runtimeSite]string{
	{goinferModule + "/internal/runtime/logs/logs.go", "logf", "os.Stderr", ""}:      "call-time stderr after the redirect",
	{goinferModule + "/internal/runtime/logs/logs.go", "writeText", "os.Stderr", ""}: "call-time stderr after the redirect",
	{decoderPkg + "/kvsnap.go", "stderrKV", "os.Stderr", ""}:                         "decoder KV snapshots only",
}

// These reads run before pbrain's main in the CUDA release archives. They
// violate the environment contract. Require the exact inventory on v0.6.4
// until the tagged fix for https://github.com/pkieltyka/goinfer/issues/70.
var knownStartupViolations = map[runtimeSite]struct{}{
	{decoderPkg + "/deltanet.go", "deltaNetTiming", "os.Getenv", `"GOINFER_DELTANET_TIMING"`}:                       {},
	{decoderPkg + "/fakequant.go", "fakeQuantScheme", "os.Getenv", `"GOINFER_FAKEQUANT"`}:                           {},
	{decoderPkg + "/fakequant.go", "fakeQuantF32Act", "os.Getenv", `"GOINFER_FAKEQUANT_ACT"`}:                       {},
	{decoderPkg + "/fakequant.go", "fakeQuantExpertsOnly", "os.Getenv", `"GOINFER_FAKEQUANT_EXPERTS"`}:              {},
	{decoderPkg + "/model.go", "decodeTiming", "os.Getenv", `"GOINFER_DECODE_TIMING"`}:                              {},
	{decoderPkg + "/weightmat.go", "w4a8SplitHalfRepackEnabled", "os.Getenv", `"GOINFER_W4A8_SPLITHALF"`}:           {},
	{decoderPkg + "/cpu_gateup_fused.go", "cpuFusedGateUp", "decoder.envBoolDefault", `"GOINFER_CPU_FUSED_GATEUP"`}: {},
	{decoderPkg + "/weightmat.go", "w4a8BatchEnabled", "decoder.envBoolDefault", `"GOINFER_W4A8_BATCH"`}:            {},
}

// The two envBoolDefault callers above each execute this same source site.
var startupHelperRead = runtimeSite{decoderPkg + "/cpu_gateup_fused.go", "envBoolDefault", "os.Getenv", "name"}

func isKnownStartupSite(site runtimeSite) bool {
	_, known := knownStartupViolations[site]
	return known || site == startupHelperRead
}
