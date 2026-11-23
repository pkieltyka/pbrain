package embed

// WriteProfile is the resolved vector identity for a sync. An fp16 writer is
// bound to one persistent device. Inventory is the same snapshot used to bind it.
type WriteProfile struct {
	Fingerprint string
	Selector    string
	DeviceID    string
	Reason      string
	Inventory   *Listing
}

const CUDAFP16Selector = "cuda-eg2-fp16"

// ResolveWriter does not reject an unavailable selector. Sync can validate
// that selector later if it needs to embed anything.
func (r *registry) ResolveWriter(m Model, request, selector string) WriteProfile {
	p := WriteProfile{Fingerprint: m.Fingerprint}
	if request != "fp16" || m.CUDAFP16Fingerprint == "" || selector == "cpu" {
		return p
	}
	e := r.entries[m.Name]
	if e.ResolveFP16 == nil || e.Devices == nil {
		return p
	}
	listing := e.Devices()
	p.Inventory = &listing
	device, reason := e.ResolveFP16(listing, selector)
	p.Reason = reason
	if device.Persistent != "" {
		p.Fingerprint = m.CUDAFP16Fingerprint
		p.Selector = CUDAFP16Selector
		p.DeviceID = device.Persistent
	}
	return p
}
