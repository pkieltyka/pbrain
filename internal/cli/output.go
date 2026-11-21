package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/render"
	"github.com/pkieltyka/pbrain/internal/secretscan"
)

type outcome struct {
	data      []byte
	err       error
	exit      int // an unsuccessful check can have a document
	committed bool
}

type versionOutput struct {
	Version string `json:"version"`
}

type helpOutput struct {
	Help string `json:"help"`
}

func helpText(cmd command) string {
	var out strings.Builder
	if cmd.name == "" {
		out.WriteString("Usage: pbrain [global options] COMMAND [options]\n\nCommands\n")
		for _, entry := range commands {
			fmt.Fprintf(&out, "  %-9s %s\n", entry.name, entry.summary)
		}
	} else {
		fmt.Fprintf(&out, "Usage: pbrain %s", cmd.name)
		if cmd.operands != "" {
			fmt.Fprintf(&out, " %s", cmd.operands)
		}
		fmt.Fprintf(&out, " [options]\n\n%s.\n", cmd.summary)
		if len(cmd.options) > 0 {
			out.WriteString("\nCommand options\n")
			printOptions(&out, cmd.options)
		}
	}
	out.WriteString("\nGlobal options\n")
	printOptions(&out, globals)
	return out.String()
}

func printOptions(out *strings.Builder, options []option) {
	for _, o := range options {
		name := "--" + o.name
		if o.value != "" {
			name += " " + o.value
		}
		fmt.Fprintf(out, "  %-20s %s\n", name, o.help)
	}
}

func encodeLine(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	return buf.Bytes(), err
}

func textOrJSON(jsonMode bool, text string, value any) outcome {
	if jsonMode {
		data, err := encodeLine(value)
		return outcome{data: data, err: err}
	}
	return outcome{data: []byte(text)}
}

type diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func emitError(w io.Writer, jsonMode bool, err error) int {
	e := domainError(err)
	emitDiagnostic(w, jsonMode, false, e.Code, e.Message)
	if e.Code == "usage" {
		return 2
	}
	return 1
}

func domainError(err error) *errs.Error {
	var e *errs.Error
	if errors.As(err, &e) && e != nil {
		return e
	}
	return ioFailure("could not complete command")
}

func emitNotice(w io.Writer, jsonMode bool, notice errs.Notice) {
	emitDiagnostic(w, jsonMode, true, notice.Code, notice.Message)
}

func emitDiagnostic(w io.Writer, jsonMode, notice bool, code, message string) {
	if w == nil {
		return
	}
	if secretscan.Sensitive(message) {
		message = "diagnostic text withheld"
	}
	message = render.OneLine(message, 0)
	if !jsonMode {
		_, _ = fmt.Fprintf(w, "%s: %s\n", code, message)
		return
	}
	d := diagnostic{Code: code, Message: message}
	value := struct {
		Notice *diagnostic `json:"notice,omitempty"`
		Error  *diagnostic `json:"error,omitempty"`
	}{}
	if notice {
		value.Notice = &d
	} else {
		value.Error = &d
	}
	line, _ := encodeLine(value)
	_, _ = w.Write(line)
}

func finish(env Env, in *invocation, result outcome) int {
	if result.err != nil && !result.committed {
		if in.cmd.name != "delete" || domainError(result.err).Code != "confirm_required" {
			return emitError(env.Stderr, in.json, result.err)
		}
	}
	if len(result.data) > 0 && env.Stdout != nil {
		n, err := env.Stdout.Write(result.data)
		if err != nil || n != len(result.data) {
			if !result.committed {
				return emitError(env.Stderr, in.json, ioFailure("could not deliver standard output"))
			}
			emitNotice(env.Stderr, in.json, errs.Notice{Code: "after_write", Message: "Could not deliver standard output. Look up the item before repeating the command."})
			return 0
		}
	}
	if result.err != nil {
		if result.committed {
			e := domainError(result.err)
			emitNotice(env.Stderr, in.json, errs.Notice{Code: "after_write", Message: e.Message})
			return 0
		}
		return emitError(env.Stderr, in.json, result.err)
	}
	return result.exit
}
