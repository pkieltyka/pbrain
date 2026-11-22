package semantic

// vectorRef names the first passage resolved for a digest. While that item has
// not been written, pending holds its vector slot. Afterwards the reference can
// read that slot's stored row without retaining the item or its vectors.
type vectorRef struct {
	pending *work
	path    string
	ord     int
}

type vectorAlias struct {
	from, to queued
}

func (b *builder) cut(w *work, title string, body []byte, queue []queued) ([]queued, error) {
	w.cut(title, body)
	for i, p := range w.ps {
		slot := queued{w, i}
		v, found, err := b.reuse(p.Digest, slot)
		if err != nil {
			return nil, err
		}
		if _, seen := b.vectors[p.Digest]; !seen {
			b.vectors[p.Digest] = vectorRef{pending: w, path: w.path, ord: i}
		}
		if found {
			w.vecs[i] = v
			w.reused++
		} else {
			queue = append(queue, slot)
		}
	}
	return queue, nil
}

func (b *builder) reuse(digest [32]byte, to queued) ([]float32, bool, error) {
	v, found, err := b.x.baseVectorByDigest(b.ctx, digest, to.w.path, b.replace)
	if err != nil {
		return nil, false, ctxOr(b.ctx, buildIOErr("the index cannot be read"))
	}
	if found {
		return v, true, nil
	}
	if first, seen := b.vectors[digest]; seen {
		if first.pending != nil {
			v := first.pending.vecs[first.ord]
			if v == nil {
				b.aliases = append(b.aliases, vectorAlias{from: queued{first.pending, first.ord}, to: to})
			}
			return v, true, nil
		}
		v, err := b.x.vectorAt(b.ctx, first.path, first.ord)
		if err != nil {
			return nil, false, ctxOr(b.ctx, buildIOErr("the index cannot be read"))
		}
		return v, true, nil
	}
	return b.si.cachedVector(digest)
}

func (b *builder) rememberStored(w *work) {
	for _, p := range w.ps {
		first := b.vectors[p.Digest]
		if first.pending == w {
			first.pending = nil
			b.vectors[p.Digest] = first
		}
	}
}
