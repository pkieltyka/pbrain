---
status: complete
---

# Audience rules

`internal/audience` decides the destination of a new item from the caller's flags, acquisition method and personal rule table. It is pure Go with no I/O. The CLI owns the audience commands, notices, output disclosure and settings transaction. Browser permission, fetching content, validating local file inputs and deciding who may see an existing item belong to their respective components.

## Interface

```go
package audience

type Destination string
const (
    Shared   Destination = "shared"
    Personal Destination = "personal"
)
type Rule struct {
    Match string      `toml:"match"`
    To    Destination `toml:"to"`
}
type Key struct {
    Host string       // canonical DNS name or IP, without brackets or port
    Port string       // empty for the effective scheme's default
    Segments []string // ordered, no terminal empty segment
}
func (k Key) String() string
func Canon(raw string, rule bool) (Key, error)
func ValidateRules(rules []Rule) ([]Rule, error)

type Acquisition uint8
const (
    Citation Acquisition = iota // add, or a file import with an optional source
    FetchedURL                  // pbrain's own public HTTP fetch
    LinkedURL                   // pointer, no fetch
)
type Request struct {
    Explicit Destination // "" = neither flag
    Source string        // citation or requested URL; never local file contents
    Method Acquisition
}
type Decision struct {
    Dest Destination
    Matched string // canonical rule key, empty for explicit flag or default
}
func Decide(rules []Rule, req Request) (Decision, error)
func Set(rules []Rule, match string, to Destination) ([]Rule, error)
func Unset(rules []Rule, match string) ([]Rule, error)
```

The package imports `errs` and `golang.org/x/net/idna` plus the standard library. It does not import `brain`, `item` or `store`. `brain.LocalConfig.Audience` uses `[]Rule`. Its loader invokes `ValidateRules` for write/management use, retaining any validation error for the caller's write preflight. CLI maps `Decision.Dest` to `store.WritePlan.Dest`. Only an explicit `--personal` widens disclosure. Both destination flags together are a CLI usage error.

## Canonical keys

Use one parser for validation, lookup and duplicate **rule** keys. A rule can be a host with optional port and path, or an absolute HTTP(S) URL. For a scheme-less rule supply HTTPS for parsing and default-port selection. For a source require a hierarchical absolute HTTP(S) URL with a nonempty host. Reject user info, malformed escapes, whitespace/control characters in the authority, invalid or empty explicit ports and unsupported schemes. Lowercase the host, use `idna.Lookup.ToASCII` for DNS names, remove the terminal dot, and normalize IP literals with `net/netip`. Serialize an IPv6 authority with brackets. Normalize a decimal port (1–65535) before removing 80 for HTTP or 443 for HTTPS. Match exact host and port, including a non-default port.

Read `url.URL.EscapedPath()`, remove its initial slash, split on literal slashes, and allow one final slash. An absent path or `/` has zero segments. Reject interior empty segments, backslashes and escaped slash/backslash. Decode unreserved ASCII escapes once, uppercase the hex of retained escapes, and reject decoded dot segments. Preserve path case. Drop query and fragment. Use the escaped form of raw non-ASCII UTF-8 bytes so raw `é` and `%C3%A9` compare equally. A DNS name rejected by IDNA Lookup (including an underscore) has no canonical audience key. `String` joins the canonical authority and segments without a trailing slash. For the retained non-default HTTP port 443, prefix `http://` in the serialized key; a bare authority would otherwise be re-read as HTTPS with a default port. Store this canonical spelling. Require `Canon(k.String(), true)` to reproduce every key, including its port. Segment-prefix matching prevents `/team` matching `/teammate`. URL canonicalization here is for audience decisions; keep the supplied locator on the item.

For shared rules validate the following minimum scopes on the canonical **host without its port**. Extra complete segments are allowed afterwards. Personal rules may stop at the host on all services.

| Host | Required shared prefix |
|---|---|
| docs.google.com | `/document/d/<id>`, `/spreadsheets/d/<id>`, or `/presentation/d/<id>` |
| drive.google.com | `/file/d/<id>` or `/drive/folders/<id>` |
| claude.ai | `/artifact/<id>` or `/code/artifact/<id>` |
| Slack workspace host ending in `.slack.com` | `/archives/<C-channel-id>`; a D/G conversation never matches a shared rule |
| github.com, gitlab.com | `/<owner>` |

Each placeholder is a nonempty complete segment. A Slack shared prefix must name the actual channel segment beginning with uppercase C. Source visibility within that channel is a decision for the importing person. Other hosts have no special minimum prefix.

`ValidateRules` canonicalizes into a fresh slice, rejects an unknown destination, overly broad shared scope or duplicate canonical key, and returns the result in ascending canonical-key order. A corrupt stored table gives `invalid_config` before any write. CLI requests with a malformed rule or impermissible scope give `invalid`; errors never echo credentials or raw private URLs. An unknown destination spelling supplied as a command operand is `usage`.

## Destination and persistence

1. Check stored rules before any write. A valid explicit flag then decides, even for an invalid audience URL. Acquisition still applies its own validation.
2. Trim surrounding ASCII spaces and tabs for classification and parsing. An HTTP(S) citation begins with `http:` or `https:` in ASCII case-insensitive comparison. `https:foo` is a URL citation with no canonical form. `task 42, https://host/x` is ordinary source text and defaults to shared. A connector citation must therefore use the URL alone. Without an explicit destination a noncanonical HTTP(S) source gives `audience_required`. No candidate rule or default may rescue it. For a non-URL citation or a file without a URL source use the shared default.
3. Among rules matching canonical host, port and whole path segments choose the longest prefix. Return its canonical key for the destination notice. A canonical tie is already an invalid configuration. With no rule, a cited HTTP(S) URL and a linked mixed-host URL require a choice. A public fetch or linked ordinary-host URL defaults to shared. Use the originally requested URL at both decision points. Public fetch redirects go through the fetch guard and never change the selected audience.

The CLI computes this before acquisition, then recomputes it from fresh settings inside the destination lock. With a changed plan, abort with `conflict` before writing. The caller alone decides what an error may disclose. For `audience_required` name only a validated host when available. Personal/private item identifiers never become suggestions on a shared write.

`pbrain audience` explicitly reads this user's own rules. List canonical keys in sorted order. Each text row is `<match>  <to>`, followed by `N rules.` (the number shown) and, for a continuation, `M more: --offset K`. Empty output is `0 rules.`. Default limit 20 (1–200), max-chars 2,000, with list paging and JSON rows `match` and `to` in `rules`, `more` and `next` only on continuation, ordinary JSON presence rules. It needs the brain and its guard, with no corpus scan or personal-folder availability dependency. `audience set <match> <to>` replaces the exact canonical key or appends one. `unset <match>` removes just that key, succeeding when absent. Both return fresh sorted slices. The CLI calls `store.Begin(ctx,b,item.Shared)` to create/open the shared root's `.pbrain/lock` and take the settings lock. Use the borrowed brain directory from `b.OpenSection(item.Shared)`. Under the lock, call `brain.EditAudience(dir sys.Dir, edit func([]audience.Rule) ([]audience.Rule,error)) (data []byte, changed bool, err error)`. It rereads and validates the complete local TOML, preserves every other TOML value including unrecognized keys, and returns a payload. For a changed result only, call `tx.ReplaceSettings(data []byte) (store.Commit,error)`; finish cleanup and close tx. No-op edits leave existing bytes/comments intact. Comments can be lost when rewriting a changed table. CLI handles both calls, preserving the package dependency direction. Text confirmation echoes canonical key and destination for set, canonical key for unset; JSON uses `match` and, for set, `to`. A failed rename leaves the previous configuration intact (`io`); failure after publication is a notice. An agent records a rule only when the person authorizes its precise scope.

### Errors and recovery

| Function | Failure and disclosure |
|---|---|
| Canon | Internal validation error with a fixed reason; the caller maps it. No raw locator. |
| ValidateRules | `invalid_config` for invalid stored entries and canonical-key collisions. Name the one-based rule position and the remedy (edit personal.toml by hand), never a stored value. |
| Decide | `invalid_config` for invalid rules; `audience_required` for an undecided citation or link; only a canonical host may appear in the latter message. |
| Set and Unset | Invalid stored table gives `invalid_config` first. Bad operand gives `invalid`; an impermissible shared scope can name the canonical host and required prefix shape only. |

Listing, setting and unsetting refuse an invalid table; ordinary item reads continue with the guarded personal path. Hand-editing the offending row restores writes. This implements the functional rule that *every* write refuses invalid rules and avoids introducing a separate raw-key edit route.

## Proving tests

- `TestCanonicalRuleSourceParity` (24–25) — IDNA, case, final dot, default and non-default ports, IPv6, escaped unreserved characters, query and fragment, invalid path and user info; round-trip every canonical rule using a fuzz test, including explicit HTTP port 443, raw/escaped é and underscore rejection. An invalid URL on an ordinary host requires a flag even for a linked item.
- `TestMixedHostsAndLongestPrefix` (22, 24–26, 28) — exercise each table row (including refusals of docs.google.com, docs.google.com/document, Docs.Google.com., acme.slack.com and acme.slack.com/archives); reject broad shared rules and shared D/G prefixes; the longest whole-segment rule wins; explicit personal wins over a valid shared rule.
- `TestDecisionBeforeAcquisition` (24–26) — test an uppercase scheme with leading whitespace, `https:foo`, a URL embedded in prose, and an ordinary URL alone. Missing audience produces no input read/cache write. A personal rule limits the duplicate details emitted without the flag.
- `TestSetCanonicalUnsetAbsent` and `TestConcurrentSettingsWriters` (24) — one canonical row after equivalent sets; no file write for absent unset; concurrent edits survive and preserve author, path and local model settings. Settings work when an external personal folder is unreachable.
- `TestAudienceListPaging` (17, 24) — stable order at limit one in both formats, empty page at end, refusal below minimum budget, no mutation or item scan. With invalid locally written rules the listing and both edit forms fail with a position and remedy while shared search succeeds.
