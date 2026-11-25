package embed

// WriteProfile is the resolved vector identity for a sync. An fp16 writer is
// bound to one persistent device. Inventory is the same snapshot used to bind it.
type WriteProfile struct {
	Fingerprint  string
	Selector     string
	DeviceID     string
	Reason       string
	Inventory    *Listing
	ResolveMetal bool // the attachment determines the vector identity
}

const CUDAFP16Selector = "cuda-eg2-fp16"

// ResolveWriter does not reject an unavailable selector. Sync can validate
// that selector later if it needs to embed anything.
func (r *registry) ResolveWriter(m Model, request, selector string) WriteProfile {
	p := WriteProfile{Fingerprint: m.Fingerprint}
	if selector == "cpu" {
		return p
	}
	e := r.entries[m.Name]
	cudaFP16 := request == "fp16" && m.CUDAFP16Fingerprint != "" && e.ResolveFP16 != nil
	hasMetal := e.MetalWriter
	auto := selector == "" || selector == "auto"
	namedMetal := selector == "metal" || selector == "metal:0"
	if e.Devices == nil || !(cudaFP16 || hasMetal && (auto || namedMetal)) {
		return p
	}
	listing := e.Devices()
	p.Inventory = &listing
	if hasMetal {
		p.ResolveMetal = namedMetal
		if auto {
			// A failed higher-ranked candidate can leave Metal as the next choice.
			for _, device := range listing.Devices {
				if device.Backend == "metal" {
					p.ResolveMetal = true
					break
				}
			}
		}
	}
	if !cudaFP16 {
		return p
	}
	device, reason := e.ResolveFP16(listing, selector)
	p.Reason = reason
	if device.Persistent != "" {
		p.ResolveMetal = false
		p.Fingerprint = m.CUDAFP16Fingerprint
		p.Selector = CUDAFP16Selector
		p.DeviceID = device.Persistent
	}
	return p
}
