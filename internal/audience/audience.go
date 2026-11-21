// Package audience: exported surface from the pbrain specs; bodies are filled in by later phases.
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
	Host     string   // canonical DNS name or IP, without brackets or port
	Port     string   // empty for the effective scheme's default
	Segments []string // ordered, no terminal empty segment
}

func (k Key) String() string                     { panic("not implemented") }
func Canon(raw string, rule bool) (Key, error)   { panic("not implemented") }
func ValidateRules(rules []Rule) ([]Rule, error) { panic("not implemented") }

type Acquisition uint8

const (
	Citation   Acquisition = iota // add, or a file import with an optional source
	FetchedURL                    // pbrain's own public HTTP fetch
	LinkedURL                     // pointer, no fetch
)

type Request struct {
	Explicit Destination // "" = neither flag
	Source   string      // citation or requested URL; never local file contents
	Method   Acquisition
}
type Decision struct {
	Dest    Destination
	Matched string // canonical rule key, empty for explicit flag or default
}

func Decide(rules []Rule, req Request) (Decision, error)             { panic("not implemented") }
func Set(rules []Rule, match string, to Destination) ([]Rule, error) { panic("not implemented") }
func Unset(rules []Rule, match string) ([]Rule, error)               { panic("not implemented") }
