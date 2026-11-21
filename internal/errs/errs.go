// Package errs: exported surface from the pbrain specs; bodies are filled in by later phases.
package errs

type Error struct {
	Code    string // one of spec 6.3's codes, no_model included
	Message string
	Exit    int // 1, or 2 for usage
}

func (e *Error) Error() string { panic("not implemented") } // so *Error is an ordinary Go error; cli finds it with errors.As

type Notice struct{ Code, Message string }
