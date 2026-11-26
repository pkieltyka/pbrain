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
		if cmd.about != "" {
			out.WriteString(cmd.about + "\n")
		}
		if len(cmd.options) > 0 {
			out.WriteString("\nCommand options\n")
			printOptions(&out, cmd.name, cmd.options)
		}
	}
	out.WriteString("\nGlobal options\n")
	printOptions(&out, cmd.name, globals)
	return out.String()
}

func printOptions(out *strings.Builder, command string, options []option) {
	for _, o := range options {
		if (command == "skill" || command == "devices") && (o.name == "brain" || o.name == "personal") {
			continue
		}
		name := "--" + o.name
		if o.value != "" {
			name += " " + o.value
		}
		fmt.Fprintf(out, "  %-20s %s%s\n", name, o.help, pageHelpDefault(command, o.name))
	}
}

func pageHelpDefault(command, flag string) string {
	_, _, def := bounds(command, flag, false, false)
	switch flag {
	case "limit", "max-chars":
		if command == "get" {
			_, _, history := bounds(command, flag, true, false)
			_, _, outline := bounds(command, flag, false, true)
			if flag == "limit" {
				return fmt.Sprintf(" (default %d with --history, %d with --outline)", history, outline)
			}
			return fmt.Sprintf(" (default %d for the body, %d with --history, %d with --outline)", def, history, outline)
		}
		return fmt.Sprintf(" (default %d)", def)
	case "offset":
		return " (default 0)"
	default:
		return ""
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
	d := Diagnostic{Code: code, Message: message}
	value := struct {
		Notice *Diagnostic `json:"notice,omitempty"`
		Error  *Diagnostic `json:"error,omitempty"`
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
			message := "Could not deliver standard output. Look up the item before repeating the command."
			if in.cmd.name == "audience" {
				message = "Could not deliver standard output. List the audience rules before repeating the command."
			}
			emitNotice(env.Stderr, in.json, errs.Notice{Code: "after_write", Message: message})
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
