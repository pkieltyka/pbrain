package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// fuzzDoc builds a list document from fuzz input: the first bytes choose the shape, the rest are
// split at NUL bytes into the texts of the head, units, alternatives, extras, hints and footer.
func fuzzDoc(data []byte) *Doc {
	shape := make([]byte, 4)
	copy(shape, data)
	pieces := strings.Split(strings.ToValidUTF8(string(data[min(len(data), 4):]), "?"), "\x00")
	next := 0
	text := func() string {
		s := strings.ReplaceAll(pieces[next%len(pieces)], "\x1b", "") // no stored escapes: colour is checked by stripping
		next++
		return s
	}
	n, offset := int(shape[0]%9), int(shape[1]%4)
	alts, extras, hints := shape[2]&1 != 0, shape[2]&2 != 0, int(shape[2]>>2)%4
	d := &Doc{Offset: offset, Total: offset + n + int(shape[3]%5)}
	if n == 0 {
		d.Total = max(0, offset-int(shape[3]%2)) // no units: an empty result, or a page past its end
	}
	head := text()
	d.Head = []Part{{Text: head, JSON: Envelope{Fields: Object{"query": head}, Rows: "results"}}}
	for i := range n {
		main := text()
		u := Unit{Main: Part{Text: main, JSON: Object{"id": main, "n": i}}}
		if main != "" {
			u.Main.Marks = []Mark{{Start: 0, End: min(3, utf8.RuneCountInString(main)), Style: Bold}}
		}
		if alts {
			a := text()
			u.Alts = []Part{{Text: main + "\n" + a, JSON: Object{"id": main, "n": i, "context": a}}}
		}
		if extras {
			for range 2 {
				e := text()
				u.Extras = append(u.Extras, Part{Text: e, JSON: Object{"id": e}})
			}
		}
		d.Units = append(d.Units, u)
	}
	if n == 0 {
		for i := range hints {
			h := text()
			d.Hints = append(d.Hints, Part{Text: h, JSON: Object{"clause": h, "count": i + 1}})
		}
	}
	foot := text()
	d.Foot = func(st State) Part {
		o := Object{}
		t := foot
		if st.More > 0 {
			t += " more"
			o["more"], o["next"] = st.More, st.Next
		}
		return Part{Text: t, JSON: o}
	}
	return d
}

// FuzzFit holds render.Fit to architecture 14's row: the output never exceeds the limit, the
// reported minimum works and one less does not, JSON is one valid document, and colour only adds
// sequences to the plain page.
func FuzzFit(f *testing.F) {
	f.Add([]byte("\x03\x01\x03\x02search\x00id-1  note\x00ctx\x00rel a\x00rel b\x00id-2\x00foot."), uint16(120), uint8(0))
	f.Add([]byte("\x00\x00\x0c\x00search: q\x00alone 1\x00alone 2\x00alone 3\x00"), uint16(40), uint8(1))
	f.Add([]byte("\x08\x02\x01\x04h\x00\"q\"\x00<&>\x00\u2028x\x00é😀\x00\x00f"), uint16(300), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, limit uint16, format uint8) {
		d := fuzzDoc(data)
		fm := []Format{Text, JSON, TextColor}[format%3]
		lim := int(limit) + 1
		out, err := Fit(d, lim, fm)
		if err != nil {
			m, ok := budgetMin(t, err)
			if !ok {
				t.Fatalf("limit %d: %v", lim, err)
			}
			if m <= lim {
				t.Fatalf("limit %d failed with the minimum %d", lim, m)
			}
			out, err = Fit(d, m, fm)
			if err != nil || utf8.RuneCount(out) > m {
				t.Fatalf("the reported minimum %d does not work: %v %d", m, err, utf8.RuneCount(out))
			}
			if _, err := Fit(d, m-1, fm); err == nil {
				t.Fatalf("one less than the minimum %d works", m)
			}
			lim = m
		}
		if n := utf8.RuneCount(out); n > lim {
			t.Fatalf("limit %d: %d code points", lim, n)
		}
		switch fm {
		case JSON:
			if !json.Valid(out) || !bytes.HasSuffix(out, []byte("}\n")) {
				t.Fatalf("not one JSON document: %q", out)
			}
		case TextColor:
			plain, err := Fit(d, lim, Text)
			if err != nil || ansi.ReplaceAllString(string(out), "") != string(plain) {
				t.Fatalf("colour is not the plain page with sequences added:\n%q\n%q %v", out, plain, err)
			}
		}
	})
}
