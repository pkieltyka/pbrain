// Package errs: exported surface from the pbrain specs; bodies are filled in by later phases.
package errs

type Error struct {
	Code    string // one of spec 6.3's codes, no_model included
	Message string
	Exit    int // 1, or 2 for usage
}

// Error returns the safe diagnostic message. CLI uses Code separately.
func (e *Error) Error() string { return e.Message }

type Notice struct{ Code, Message string }
