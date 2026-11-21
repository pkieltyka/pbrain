package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"regexp"
	"strconv"

	_ "modernc.org/sqlite"     // the SQLite driver, registered as "sqlite"
	_ "modernc.org/sqlite/vec" // sqlite-vec, loaded into every connection

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// IndexState is what reading a section's index found.
type IndexState uint8

const (
	IndexOK         IndexState = iota
	IndexNone                  // no manifest
	IndexStale                 // built for another fingerprint or dimension
	IndexUnreadable            // anything else that is not a usable index
)

// maxDatabaseSize is the largest database file, 2,147,483,647 bytes; a variable for the tests.
var maxDatabaseSize int64 = math.MaxInt32

const (
	indexFormat  = 2
	manifestName = "manifest"
	maxManifest  = 4096
	// cutoffMargin widens the query's distance bound so that search, which applies the cut-off
	// exactly, sees every passage at or just under it.
	cutoffMargin = 0.000001
)

var (
	dbName  = regexp.MustCompile(`^[0-9a-f]{16}\.db$`)
	hex64   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	errBad  = errors.New("semantic: not a usable index")
	errGone = errors.New("semantic: the database file is gone")
)

// manifest is index/manifest: which database file is the index, and what it must be.
type manifest struct {
	Format      int    `json:"format"`
	Fingerprint string `json:"fingerprint"`
	Model       string `json:"model"`
	Dim         int    `json:"dim"`
	DB          string `json:"db"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
}

// scoreSQL is the one scoring query (semantic.md, "Scoring"). CROSS JOIN makes SQLite start from
// eligible, so only the passages of listed items are ever compared.
const scoreSQL = `SELECT item_id, ord, start, stop, d
FROM (SELECT p.item_id, p.ord, p.start, p.stop, vec_distance_cosine(p.vector, ?1) AS d
      FROM temp.eligible e CROSS JOIN passage p ON p.item_id = e.id)
WHERE d <= ?2`

// indexDB is one index held in a private in-memory SQLite database. SQLite is never given a path.
type indexDB struct {
	db          *sql.DB
	conn        *sql.Conn
	dim         int
	fingerprint string
	model       string
	items       map[string]storedItem // path -> row
	score       *sql.Stmt
}

type storedItem struct {
	id      int64
	content [32]byte
}

// storedPassage is one passage row.
type storedPassage struct {
	Ord, Start, Stop int
	Digest           [32]byte
	Vector           []float32
}

// hitRow is one row of the scoring query.
type hitRow struct {
	ItemID           int64
	Ord, Start, Stop int
	Distance         float64
}

// memoryDB opens one private connection to an in-memory database.
func memoryDB(ctx context.Context) (*indexDB, error) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	x := &indexDB{db: db, conn: conn, items: map[string]storedItem{}}
	if err := x.pragmas(ctx); err != nil {
		x.Close()
		return nil, err
	}
	return x, nil
}

func (x *indexDB) pragmas(ctx context.Context) error {
	for _, p := range []string{"PRAGMA journal_mode=MEMORY", "PRAGMA temp_store=MEMORY", "PRAGMA foreign_keys=ON"} {
		if _, err := x.conn.ExecContext(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the database.
func (x *indexDB) Close() error {
	if x.score != nil {
		x.score.Close()
	}
	x.conn.Close()
	return x.db.Close()
}

// newIndex creates an empty index for fingerprint, model and dim, by the schema of semantic.md.
func newIndex(ctx context.Context, fingerprint, model string, dim int) (*indexDB, error) {
	if dim < 1 || dim > 1<<16 {
		return nil, fmt.Errorf("semantic: dimension %d", dim)
	}
	x, err := memoryDB(ctx)
	if err != nil {
		return nil, err
	}
	schema := []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID`,
		`CREATE TABLE item (
    id      INTEGER PRIMARY KEY,
    path    TEXT NOT NULL UNIQUE,
    content BLOB NOT NULL CHECK (length(content) = 32)
)`,
		fmt.Sprintf(`CREATE TABLE passage (
    id      INTEGER PRIMARY KEY,
    item_id INTEGER NOT NULL REFERENCES item(id) ON DELETE CASCADE,
    ord     INTEGER NOT NULL,
    start   INTEGER NOT NULL,
    stop    INTEGER NOT NULL CHECK (stop >= start),
    digest  BLOB NOT NULL CHECK (length(digest) = 32),
    vector  BLOB NOT NULL CHECK (length(vector) = %d),
    UNIQUE (item_id, ord)
)`, 4*dim),
		`CREATE INDEX passage_digest ON passage(digest)`,
	}
	for _, s := range schema {
		if _, err := x.conn.ExecContext(ctx, s); err != nil {
			x.Close()
			return nil, err
		}
	}
	for k, v := range map[string]string{"format": strconv.Itoa(indexFormat), "fingerprint": fingerprint, "model": model, "dim": strconv.Itoa(dim)} {
		if _, err := x.conn.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?)`, k, v); err != nil {
			x.Close()
			return nil, err
		}
	}
	x.dim, x.fingerprint, x.model = dim, fingerprint, model
	if err := x.prepare(ctx); err != nil {
		x.Close()
		return nil, err
	}
	return x, nil
}

// prepare creates the eligible table and the scoring statement; either fails on a database that
// lacks a table or column the query needs.
func (x *indexDB) prepare(ctx context.Context) error {
	if _, err := x.conn.ExecContext(ctx, `CREATE TEMP TABLE IF NOT EXISTS eligible (id INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	stmt, err := x.conn.PrepareContext(ctx, scoreSQL)
	if err != nil {
		return err
	}
	x.score = stmt
	return nil
}

func vectorBlob(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}

// putItem replaces the item at path, with all its passages, in one transaction: an item is in the
// index with all its old passages or all its new ones, never half.
func (x *indexDB) putItem(ctx context.Context, path string, content [32]byte, passages []storedPassage) error {
	tx, err := x.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // after Commit it does nothing
	if _, err := tx.ExecContext(ctx, `DELETE FROM item WHERE path = ?`, path); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO item (path, content) VALUES (?, ?)`, path, content[:])
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, p := range passages {
		if len(p.Vector) != x.dim {
			return fmt.Errorf("semantic: a vector of %d values for dimension %d", len(p.Vector), x.dim)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO passage (item_id, ord, start, stop, digest, vector) VALUES (?, ?, ?, ?, ?, ?)`,
			id, p.Ord, p.Start, p.Stop, p.Digest[:], vectorBlob(p.Vector)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	x.items[path] = storedItem{id: id, content: content}
	return nil
}

// removeItem drops the item at path and its passages.
func (x *indexDB) removeItem(ctx context.Context, path string) error {
	if _, err := x.conn.ExecContext(ctx, `DELETE FROM item WHERE path = ?`, path); err != nil {
		return err
	}
	delete(x.items, path)
	return nil
}

// removeItems drops the items at paths and their passages in one transaction.
func (x *indexDB) removeItems(ctx context.Context, paths []string) error {
	tx, err := x.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // after Commit it does nothing
	for _, p := range paths {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item WHERE path = ?`, p); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, p := range paths {
		delete(x.items, p)
	}
	return nil
}

// vectorByDigest returns a stored vector of a passage with this digest, from any row, the rows of
// items gone from the section included; a row whose vector is not a unit vector of dim values is
// passed over.
func (x *indexDB) vectorByDigest(ctx context.Context, digest [32]byte) ([]float32, bool, error) {
	rows, err := x.conn.QueryContext(ctx, `SELECT vector FROM passage WHERE digest = ?`, digest[:])
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, false, err
		}
		if len(blob) != 4*x.dim {
			continue
		}
		v := make([]float32, x.dim)
		for i := range v {
			v[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[4*i:]))
		}
		if validVector(v, x.dim) {
			return v, true, nil
		}
	}
	return nil, false, rows.Err()
}

// passagesOf returns an item's stored passages in order.
func (x *indexDB) passagesOf(ctx context.Context, id int64) ([]storedPassage, error) {
	rows, err := x.conn.QueryContext(ctx, `SELECT ord, start, stop, digest, vector FROM passage WHERE item_id = ? ORDER BY ord`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storedPassage
	for rows.Next() {
		var p storedPassage
		var digest, vec []byte
		if err := rows.Scan(&p.Ord, &p.Start, &p.Stop, &digest, &vec); err != nil {
			return nil, err
		}
		if len(digest) != 32 || len(vec) != 4*x.dim {
			return nil, errBad
		}
		copy(p.Digest[:], digest)
		p.Vector = make([]float32, x.dim)
		for i := range p.Vector {
			p.Vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(vec[4*i:]))
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// scoreItems runs the scoring query over the items ids with the query vector q, returning every
// passage within 1 - cutoff + margin.
func (x *indexDB) scoreItems(ctx context.Context, ids []int64, q []float32, cutoff float32) ([]hitRow, error) {
	if len(q) != x.dim {
		return nil, fmt.Errorf("semantic: a query of %d values for dimension %d", len(q), x.dim)
	}
	tx, err := x.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM temp.eligible`); err != nil {
		return nil, err
	}
	ins, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO temp.eligible (id) VALUES (?)`)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := ins.ExecContext(ctx, id); err != nil {
			ins.Close()
			return nil, err
		}
	}
	ins.Close()
	rows, err := tx.StmtContext(ctx, x.score).QueryContext(ctx, vectorBlob(q), 1-float64(cutoff)+cutoffMargin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []hitRow
	for rows.Next() {
		var h hitRow
		if err := rows.Scan(&h.ItemID, &h.Ord, &h.Start, &h.Stop, &h.Distance); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

// serialize returns the database as one file's bytes.
func (x *indexDB) serialize(ctx context.Context) ([]byte, error) {
	var out []byte
	err := x.conn.Raw(func(dc any) error {
		s, ok := dc.(interface{ Serialize() ([]byte, error) })
		if !ok {
			return errors.New("semantic: the driver cannot serialize")
		}
		var err error
		out, err = s.Serialize()
		return err
	})
	return out, err
}

func (x *indexDB) deserialize(ctx context.Context, b []byte) error {
	return x.conn.Raw(func(dc any) error {
		d, ok := dc.(interface{ Deserialize([]byte) error })
		if !ok {
			return errors.New("semantic: the driver cannot deserialize")
		}
		return d.Deserialize(b)
	})
}

// readManifest reads and checks index/manifest; nothing in it is used before it is checked.
func readManifest(files sys.Files, dir sys.Dir) (manifest, IndexState) {
	f, err := files.OpenRegular(dir, manifestName)
	if errors.Is(err, fs.ErrNotExist) {
		return manifest{}, IndexNone
	}
	if err != nil {
		return manifest{}, IndexUnreadable
	}
	data, err := io.ReadAll(io.LimitReader(f, maxManifest+1))
	f.Close()
	if err != nil || len(data) > maxManifest {
		return manifest{}, IndexUnreadable
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return manifest{}, IndexUnreadable
	}
	// Each number must be a JSON number of integer form and each string a JSON string: "2" is not 2.
	integer := func(k string) (int64, bool) {
		n, err := strconv.ParseInt(string(raw[k]), 10, 64)
		return n, err == nil
	}
	text := func(k string) (string, bool) {
		v, ok := raw[k]
		if !ok || len(v) == 0 || v[0] != '"' {
			return "", false
		}
		var s string
		return s, json.Unmarshal(v, &s) == nil
	}
	format, ok1 := integer("format")
	dim, ok2 := integer("dim")
	size, ok3 := integer("size")
	fingerprint, ok4 := text("fingerprint")
	model, ok5 := text("model")
	db, ok6 := text("db")
	sum, ok7 := text("sha256")
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 || format != indexFormat || dim < 1 || dim > 1<<16 {
		return manifest{}, IndexUnreadable
	}
	if !dbName.MatchString(db) || !hex64.MatchString(sum) || size < 1 || size > maxDatabaseSize {
		return manifest{}, IndexUnreadable
	}
	m := manifest{Format: int(format), Fingerprint: fingerprint, Model: model, Dim: int(dim), DB: db, Size: size, SHA256: sum}
	return m, IndexOK
}

// readDatabase reads exactly m.Size bytes of m.DB, after the open file's own size agrees, and
// checks their SHA-256. Nothing is allocated for a size the file does not have.
func readDatabase(files sys.Files, dir sys.Dir, m manifest) ([]byte, error) {
	f, err := files.OpenRegular(dir, m.DB)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errGone
	}
	if err != nil {
		return nil, errBad
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size != m.Size {
		return nil, errBad
	}
	data := make([]byte, m.Size)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, errBad
	}
	if n, _ := f.Read(make([]byte, 1)); n != 0 {
		return nil, errBad
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != m.SHA256 {
		return nil, errBad
	}
	return data, nil
}

// openIndex reads a section's index from its index folder (semantic.md, "Reading"). With full set
// (the build and the status) every stored row is checked as well.
func openIndex(ctx context.Context, files sys.Files, dir sys.Dir, fingerprint string, dim int, full bool) (*indexDB, IndexState) {
	m, state := readManifest(files, dir)
	if state != IndexOK {
		return nil, state
	}
	if m.Fingerprint != fingerprint || m.Dim != dim {
		return nil, IndexStale
	}
	data, err := readDatabase(files, dir, m)
	if errors.Is(err, errGone) {
		// A build may have published since the manifest was read: read it once more.
		m2, state := readManifest(files, dir)
		if state != IndexOK {
			return nil, state
		}
		if m2.Fingerprint != fingerprint || m2.Dim != dim {
			return nil, IndexStale
		}
		if m2 == m {
			return nil, IndexUnreadable
		}
		m = m2
		data, err = readDatabase(files, dir, m)
	}
	if err != nil {
		return nil, IndexUnreadable
	}
	x, err := memoryDB(ctx)
	if err != nil {
		return nil, IndexUnreadable
	}
	ok := func() bool {
		if err := x.deserialize(ctx, data); err != nil {
			return false
		}
		data = nil // SQLite holds its own copy
		if err := x.pragmas(ctx); err != nil {
			return false
		}
		meta := map[string]string{}
		rows, err := x.conn.QueryContext(ctx, `SELECT key, value FROM meta`)
		if err != nil {
			return false
		}
		for rows.Next() {
			var k, v string
			if rows.Scan(&k, &v) != nil {
				rows.Close()
				return false
			}
			meta[k] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false
		}
		if meta["format"] != strconv.Itoa(indexFormat) || meta["fingerprint"] != m.Fingerprint ||
			meta["model"] != m.Model || meta["dim"] != strconv.Itoa(m.Dim) {
			return false
		}
		x.dim, x.fingerprint, x.model = m.Dim, m.Fingerprint, m.Model
		if x.prepare(ctx) != nil {
			return false
		}
		items, err := x.conn.QueryContext(ctx, `SELECT id, path, content FROM item`)
		if err != nil {
			return false
		}
		for items.Next() {
			var id int64
			var path string
			var content []byte
			if items.Scan(&id, &path, &content) != nil || len(content) != 32 {
				items.Close()
				return false
			}
			var c [32]byte
			copy(c[:], content)
			x.items[path] = storedItem{id: id, content: c}
		}
		err = items.Err()
		items.Close()
		if err != nil {
			return false
		}
		if !full {
			return true
		}
		return x.checkRows(ctx)
	}()
	if !ok {
		x.Close()
		return nil, IndexUnreadable
	}
	return x, IndexOK
}

// checkRows is the full check of the build and the status: every vector of the right length, no
// stop below start, no passage without its item, and the scoring query run once over every item.
func (x *indexDB) checkRows(ctx context.Context) bool {
	var bad int
	err := x.conn.QueryRowContext(ctx, `SELECT count(*) FROM passage p LEFT JOIN item i ON i.id = p.item_id
WHERE typeof(p.vector) != 'blob' OR length(p.vector) != ? OR p.stop < p.start OR i.id IS NULL`, 4*x.dim).Scan(&bad)
	if err != nil || bad != 0 {
		return false
	}
	ids := make([]int64, 0, len(x.items))
	for _, it := range x.items {
		ids = append(ids, it.id)
	}
	unit := make([]float32, x.dim)
	unit[0] = 1
	_, err = x.scoreItems(ctx, ids, unit, -1)
	return err == nil
}

// publish writes the database as a new file of index/ and points the manifest at it (semantic.md,
// "Publishing"). The previous database file is removed after the manifest names the new one; only
// a name of the generated form is ever removed. A failure before the rename leaves the old index.
func (x *indexDB) publish(ctx context.Context, files sys.Files, dir sys.Dir, rnd io.Reader) (manifest, error) {
	data, err := x.serialize(ctx)
	if err != nil {
		return manifest{}, ioErr("the index cannot be written")
	}
	if int64(len(data)) > maxDatabaseSize {
		return manifest{}, &errs.Error{Code: "too_large", Message: "the index would pass 2 GiB", Exit: 1}
	}
	old, _ := readManifest(files, dir)
	tmp, err := stageFile(files, dir, data)
	if err != nil {
		return manifest{}, err
	}
	var name string
	for range 16 {
		var b [8]byte
		if _, err := io.ReadFull(rnd, b[:]); err != nil {
			dir.Remove(tmp) //nolint:errcheck
			return manifest{}, ioErr("no random name for the index")
		}
		name = hex.EncodeToString(b[:]) + ".db"
		if err = dir.LinkNew(tmp, name); err == nil || !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	dir.Remove(tmp) //nolint:errcheck
	if err != nil {
		return manifest{}, ioErr("the index cannot be written")
	}
	if err := dir.Sync(); err != nil {
		return manifest{}, ioErr("the index cannot be written")
	}
	sum := sha256.Sum256(data)
	m := manifest{Format: indexFormat, Fingerprint: x.fingerprint, Model: x.model, Dim: x.dim, DB: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
	mdata, _ := json.Marshal(m)
	mtmp, err := stageFile(files, dir, mdata)
	if err != nil {
		return manifest{}, err
	}
	if err := dir.Replace(mtmp, manifestName); err != nil {
		dir.Remove(mtmp) //nolint:errcheck
		return manifest{}, ioErr("the index cannot be written")
	}
	if err := dir.Sync(); err != nil {
		return manifest{}, ioErr("the index cannot be written")
	}
	if old.DB != "" && old.DB != name && dbName.MatchString(old.DB) {
		dir.Remove(old.DB) //nolint:errcheck // a stale file is removed by a later build
	}
	return m, nil
}

func ioErr(msg string) error { return &errs.Error{Code: "io", Message: msg, Exit: 1} }

// stageFile writes data to a flushed temporary file of dir.
func stageFile(files sys.Files, dir sys.Dir, data []byte) (string, error) {
	tmp, w, err := dir.CreateTemp()
	if err != nil {
		return "", ioErr("the index cannot be written")
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		dir.Remove(tmp) //nolint:errcheck
		return "", ioErr("the index cannot be written")
	}
	if err := files.Flush(w); err != nil {
		dir.Remove(tmp) //nolint:errcheck
		return "", ioErr("the index cannot be written")
	}
	return tmp, nil
}
