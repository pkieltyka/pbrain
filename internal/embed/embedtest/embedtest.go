// Package embedtest is the test registry of semantic.md's test plan: models of 8 dimensions with
// cut-off 0.5 whose embedder is built from the verified bytes of a keyword table, and a fake
// Downloader that serves the pinned files. It is imported by tests only.
package embedtest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
)

const (
	Dim    = 8
	Cutoff = 0.5
	// The names of the kit's models.
	ModelA      = "test-model"        // one file
	ModelB      = "test-model-b"      // one file, another fingerprint
	ModelPair   = "test-model-pair"   // two files, both needed
	ModelPacked = "test-model-packed" // one file derived from one source file
)

// DefaultTable is the keyword table used when New is given none.
var DefaultTable = map[string][]float32{
	"alpha": {1, 0, 0, 0, 0, 0, 0, 0},
	"beta":  {0, 1, 0, 0, 0, 0, 0, 0},
	"gamma": {0, 0, 1, 0, 0, 0, 0, 0},
	"delta": {0, 0, 0, 1, 0, 0, 0, 0},
}

// Kit holds the test models, their pinned bytes and what the embedders were asked.
type Kit struct {
	Registry embed.Registry
	Models   map[string]embed.Model
	// Bytes are the pinned bytes of every file and source file, by URL.
	Bytes map[string][]byte

	// Opens counts embedders opened, Closes those closed, Calls the Embed calls and Texts the texts.
	Opens, Closes, Calls, Texts atomic.Int64
}

// New builds a kit over table, or DefaultTable when table is nil. Every vector has Dim values.
func New(table map[string][]float32) *Kit {
	if table == nil {
		table = DefaultTable
	}
	k := &Kit{Models: map[string]embed.Model{}, Bytes: map[string][]byte{}}
	var entries []embed.Entry
	add := func(name string, files map[string][]byte, order []string, source []embed.File, derive func([][]byte) ([][]byte, error)) {
		m := embed.Model{Name: name, Runtime: "embedtest", Dim: Dim, Cutoff: Cutoff, Source: source}
		for _, n := range order {
			m.Files = append(m.Files, k.pin(name, n, files[n]))
		}
		fp := sha256.New()
		fp.Write([]byte("embedtest\n" + name + "\n"))
		for _, f := range m.Files {
			fp.Write(f.SHA256[:])
		}
		m.Fingerprint = hex.EncodeToString(fp.Sum(nil))
		k.Models[name] = m
		entries = append(entries, embed.Entry{Model: m, Open: k.open, Derive: derive})
	}
	add(ModelA, map[string][]byte{"keywords.txt": tableBytes("a", table)}, []string{"keywords.txt"}, nil, nil)
	add(ModelB, map[string][]byte{"keywords.txt": tableBytes("b", table)}, []string{"keywords.txt"}, nil, nil)
	add(ModelPair, map[string][]byte{"keywords.txt": tableBytes("pair", table), "settings.txt": []byte("dim 8\n")},
		[]string{"keywords.txt", "settings.txt"}, nil, nil)
	src := sourceBytes(table)
	packed, _ := Derive([][]byte{src})
	add(ModelPacked, map[string][]byte{"keywords.txt": packed[0]}, []string{"keywords.txt"},
		[]embed.File{k.pin(ModelPacked, "keywords.src", src)}, Derive)
	k.Registry = embed.NewRegistry(entries...)
	return k
}

func (k *Kit) pin(model, name string, data []byte) embed.File {
	url := "https://models.test/" + model + "/" + name
	k.Bytes[url] = data
	return embed.File{Name: name, URL: url, SHA256: sha256.Sum256(data), Size: int64(len(data))}
}

// tableBytes writes a table as the model file: a header line naming the variant, then one line per
// keyword in sorted order.
func tableBytes(variant string, table map[string][]float32) []byte {
	var b bytes.Buffer
	b.WriteString("embedtest " + variant + "\n")
	for _, w := range sortedKeys(table) {
		b.WriteString(w)
		for _, f := range table[w] {
			b.WriteString(" " + strconv.FormatFloat(float64(f), 'g', -1, 32))
		}
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// sourceBytes is the published form of the packed model: its lines in reverse order.
func sourceBytes(table map[string][]float32) []byte {
	lines := strings.Split(strings.TrimSuffix(string(tableBytes("packed", table)), "\n"), "\n")
	slices.Reverse(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// Derive is the packed model's Derive: it puts the source's lines back in order.
func Derive(source [][]byte) ([][]byte, error) {
	if len(source) != 1 {
		return nil, fmt.Errorf("embedtest: %d source files", len(source))
	}
	lines := strings.Split(strings.TrimSuffix(string(source[0]), "\n"), "\n")
	slices.Reverse(lines)
	return [][]byte{[]byte(strings.Join(lines, "\n") + "\n")}, nil
}

func sortedKeys(table map[string][]float32) []string {
	keys := make([]string, 0, len(table))
	for w := range table {
		keys = append(keys, w)
	}
	slices.Sort(keys)
	return keys
}

// open builds an embedder from the artifact's bytes alone, after hashing them against the pins.
func (k *Kit) open(ctx context.Context, a *embed.Artifact) (embed.Embedder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i, f := range a.Model.Files {
		if sha256.Sum256(a.Files[i]) != f.SHA256 {
			return nil, fmt.Errorf("embedtest: %s is not the pinned file", f.Name)
		}
	}
	table := map[string][]float32{}
	sc := bufio.NewScanner(bytes.NewReader(a.Files[0]))
	sc.Scan() // the header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != Dim+1 {
			return nil, errors.New("embedtest: a malformed table line")
		}
		v := make([]float32, Dim)
		for i := range v {
			f, err := strconv.ParseFloat(fields[i+1], 32)
			if err != nil {
				return nil, err
			}
			v[i] = float32(f)
		}
		table[fields[0]] = v
	}
	k.Opens.Add(1)
	return &embedder{kit: k, table: table, keys: sortedKeys(table), fingerprint: a.Model.Fingerprint}, nil
}

type embedder struct {
	kit         *Kit
	table       map[string][]float32
	keys        []string
	fingerprint string
	closed      atomic.Bool
}

// Embed gives each text the normalised sum of the vectors of the keywords it contains, or the
// last unit vector when it contains none (or they cancel out).
func (e *embedder) Embed(ctx context.Context, _ embed.Role, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.kit.Calls.Add(1)
	e.kit.Texts.Add(int64(len(texts)))
	out := make([][]float32, len(texts))
	for i, t := range texts {
		lower := strings.ToLower(t)
		v := make([]float32, Dim)
		for _, w := range e.keys {
			if strings.Contains(lower, w) {
				for j, f := range e.table[w] {
					v[j] += f
				}
			}
		}
		var n float64
		for _, f := range v {
			n += float64(f) * float64(f)
		}
		if n == 0 {
			v[Dim-1] = 1
		} else {
			for j := range v {
				v[j] = float32(float64(v[j]) / math.Sqrt(n))
			}
		}
		out[i] = v
	}
	return out, nil
}

func (e *embedder) Dim() int            { return Dim }
func (e *embedder) Fingerprint() string { return e.fingerprint }
func (e *embedder) Close() error {
	if e.closed.CompareAndSwap(false, true) {
		e.kit.Closes.Add(1)
	}
	return nil
}

// Downloader is a fake embed.Downloader. It serves Bodies by URL as fetch.Client would: at most
// maxBytes are written, and a body past them fails with fetch_failed after the first maxBytes.
type Downloader struct {
	mu       sync.Mutex
	bodies   map[string][]byte
	requests map[string]int
	// Before, when set, is called before a body is served; a test pauses or acts there.
	Before func(url string)
	// Err, when set, is returned for every request.
	Err error
}

// Downloader returns a fake that serves the kit's pinned bytes.
func (k *Kit) Downloader() *Downloader {
	d := &Downloader{bodies: map[string][]byte{}, requests: map[string]int{}}
	for url, b := range k.Bytes {
		d.bodies[url] = b
	}
	return d
}

// Serve replaces the body served for url.
func (d *Downloader) Serve(url string, body []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bodies[url] = body
}

// Requests returns how often url was asked for; with no url, the total.
func (d *Downloader) Requests(url ...string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for u, c := range d.requests {
		if len(url) == 0 || slices.Contains(url, u) {
			n += c
		}
	}
	return n
}

func (d *Downloader) Download(ctx context.Context, rawURL string, dst io.Writer, maxBytes int64, _ time.Duration) (int64, error) {
	d.mu.Lock()
	d.requests[rawURL]++
	body, ok := d.bodies[rawURL]
	before, fail := d.Before, d.Err
	d.mu.Unlock()
	if before != nil {
		before(rawURL)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if fail != nil {
		return 0, fail
	}
	failed := &errs.Error{Code: "fetch_failed", Message: "download failed", Exit: 1}
	if !ok {
		return 0, failed
	}
	n, err := dst.Write(body[:min(int64(len(body)), maxBytes)])
	if err != nil {
		return int64(n), &errs.Error{Code: "io", Message: "could not store downloaded data", Exit: 1}
	}
	if int64(len(body)) > maxBytes {
		return int64(n), failed
	}
	return int64(n), nil
}
