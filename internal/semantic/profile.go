package semantic

import (
	"sync"

	"github.com/pkieltyka/pbrain/internal/embed"
)

// ShareWriterResolution lets one sync use a single lazy device snapshot across
// sections. Call it after setting the model and device options.
func (o *Options) ShareWriterResolution() {
	reg, name, request, device := o.Registry, o.ModelName, o.CUDAPrecision, o.Device
	o.writer = sync.OnceValue(func() embed.WriteProfile {
		model, _ := reg.Lookup(name)
		return embed.ResolveWriter(reg, model, request, device)
	})
}

func precisionMarker(model embed.Model, request string) string {
	if request == "fp16" && model.CUDAFP16Fingerprint != "" {
		return request
	}
	return ""
}

// manifestProfile returns the effective vector identity, or empty when the
// manifest is stale under this request. This needs no device discovery.
func manifestProfile(m manifest, model embed.Model, request string) string {
	marker := precisionMarker(model, request)
	if m.Model != model.Name || m.Dim != model.Dim || m.CUDAPrecisionRequest != marker {
		return ""
	}
	switch {
	case m.Fingerprint == IndexFingerprint(model):
		return model.Fingerprint
	case marker == "fp16" && m.Fingerprint == indexFingerprint(model.CUDAFP16Fingerprint):
		return model.CUDAFP16Fingerprint
	default:
		return ""
	}
}

func (b *builder) resolveWriter() embed.WriteProfile {
	if len(b.in.Items) != 0 {
		if b.opt.writer != nil {
			return b.opt.writer()
		}
		return embed.ResolveWriter(b.opt.Registry, b.m, b.opt.CUDAPrecision, b.opt.Device)
	}
	old, state := readManifest(b.opt.Files, b.idx)
	if state == IndexOK {
		if fp := manifestProfile(old, b.m, b.opt.CUDAPrecision); fp != "" {
			return embed.WriteProfile{Fingerprint: fp}
		}
	}
	return embed.WriteProfile{Fingerprint: b.m.Fingerprint}
}
