package brain

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// DefaultModel is the release model used when pbrain.toml omits semantic.model.
const DefaultModel = "embeddinggemma-2"

// The defaults and flag ranges of spec 6.1.
const (
	defaultSearchLimit, maxSearchLimit = 5, 50
	defaultSearchChars                 = 3000
	defaultListLimit, maxListLimit     = 20, 200
	defaultListChars                   = 2000
	defaultGetChars                    = 6000
	defaultPrimeChars                  = 2000
	maxActorChars                      = 200
)

var tagName = regexp.MustCompile(`^[a-z0-9-]+$`)

func invalidConfig(format string, a ...any) error {
	return &errs.Error{Code: "invalid_config", Message: fmt.Sprintf(format, a...), Exit: 1}
}

type limitsWire struct {
	Limit    *int `toml:"limit"`
	MaxChars *int `toml:"max_chars"`
}

type charsWire struct {
	MaxChars *int `toml:"max_chars"`
}

type projectWire struct {
	Search   *limitsWire `toml:"search"`
	List     *limitsWire `toml:"list"`
	Get      *charsWire  `toml:"get"`
	Prime    *charsWire  `toml:"prime"`
	Semantic *struct {
		Model *string `toml:"model"`
	} `toml:"semantic"`
	Tags map[string]string `toml:"tags"`
}

// decodeProject reads pbrain.toml (spec 10). Every key is optional; an absent key takes the
// default of spec 6.1, a supplied one must be in range. Unknown keys are ignored.
func decodeProject(data []byte) (Config, error) {
	var w projectWire
	md, err := toml.Decode(string(data), &w)
	if err != nil {
		return Config{}, invalidConfig("pbrain.toml does not parse: %s", tomlPosition(err))
	}
	c := Config{
		Search: Limits{Limit: defaultSearchLimit, MaxChars: defaultSearchChars},
		List:   Limits{Limit: defaultListLimit, MaxChars: defaultListChars},
	}
	c.Get.MaxChars, c.Prime.MaxChars = defaultGetChars, defaultPrimeChars
	limits := func(name string, l *limitsWire, into *Limits, maxLimit int) error {
		if l == nil {
			return nil
		}
		if l.Limit != nil {
			if *l.Limit < 1 || *l.Limit > maxLimit {
				return invalidConfig("pbrain.toml: %s.limit must be from 1 to %d", name, maxLimit)
			}
			into.Limit = *l.Limit
		}
		return chars(name, l.MaxChars, &into.MaxChars)
	}
	if err := limits("search", w.Search, &c.Search, maxSearchLimit); err != nil {
		return Config{}, err
	}
	if err := limits("list", w.List, &c.List, maxListLimit); err != nil {
		return Config{}, err
	}
	if w.Get != nil {
		if err := chars("get", w.Get.MaxChars, &c.Get.MaxChars); err != nil {
			return Config{}, err
		}
	}
	if w.Prime != nil {
		if err := chars("prime", w.Prime.MaxChars, &c.Prime.MaxChars); err != nil {
			return Config{}, err
		}
	}
	c.Semantic.Model = DefaultModel
	if w.Semantic != nil && w.Semantic.Model != nil {
		switch model := *w.Semantic.Model; model {
		case "":
			return Config{}, invalidConfig("pbrain.toml: semantic.model cannot be empty")
		case "none":
			c.Semantic.Model = ""
		default:
			c.Semantic.Model = model
		}
	}
	if md.IsDefined("tags") {
		c.HasTags = true
		c.Tags = map[string]string{}
		for name, desc := range w.Tags {
			if !tagName.MatchString(name) {
				return Config{}, invalidConfig("pbrain.toml: a tag name in [tags] is not a-z, 0-9 and -")
			}
			if strings.TrimSpace(desc) == "" || strings.ContainsAny(desc, "\r\n") {
				return Config{}, invalidConfig("pbrain.toml: the description of tag %s must be one non-empty line", name)
			}
			c.Tags[name] = desc
		}
	}
	return c, nil
}

func chars(name string, v *int, into *int) error {
	if v == nil {
		return nil
	}
	if *v < 1 {
		return invalidConfig("pbrain.toml: %s.max_chars must be a positive number", name)
	}
	*into = *v
	return nil
}

// tomlPosition is a parse error's position, never the text around it.
func tomlPosition(err error) string {
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return fmt.Sprintf("line %d", pe.Position.Line)
	}
	return "a value has the wrong type"
}

// local is personal.toml as read: the settings, and the audience rules' own error, kept apart so
// that an invalid rule leaves the brain readable (brain.md, "Discovery and configuration").
type local struct {
	config   LocalConfig
	rulesErr error
	raw      map[string]any // every key, for a rewrite that keeps them
}

func stringKey(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", invalidConfig("personal.toml: %s must be a string", key)
	}
	return s, nil
}

// decodeLocal reads personal.toml. Syntax, path, author and model_path fail at once; an invalid
// audience value or rule is returned apart, in rulesErr.
func decodeLocal(data []byte) (local, error) {
	raw := map[string]any{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return local{}, invalidConfig("personal.toml does not parse: %s", tomlPosition(err))
	}
	var l local
	l.raw = raw
	var err error
	if l.config.Path, err = stringKey(raw, "path"); err != nil {
		return local{}, err
	}
	if l.config.Author, err = stringKey(raw, "author"); err != nil {
		return local{}, err
	}
	if l.config.ModelPath, err = stringKey(raw, "model_path"); err != nil {
		return local{}, err
	}
	rules, err := rulesOf(raw["audience"])
	if err == nil {
		rules, err = audience.ValidateRules(rules)
	}
	if err != nil {
		l.rulesErr = invalidConfig("personal.toml: the audience rules are not valid; fix them by hand")
		var e *errs.Error
		if errors.As(err, &e) && e.Code == "invalid_config" {
			l.rulesErr = e
		}
	} else {
		l.config.Audience = rules
	}
	return l, nil
}

// rulesOf converts the decoded [[audience]] value without validating the rules themselves.
func rulesOf(v any) ([]audience.Rule, error) {
	if v == nil {
		return nil, nil
	}
	var list []map[string]any
	switch t := v.(type) {
	case []map[string]any:
		list = t
	case []any:
		for _, e := range t {
			m, ok := e.(map[string]any)
			if !ok {
				return nil, errors.New("not a table")
			}
			list = append(list, m)
		}
	default:
		return nil, errors.New("not an array of tables")
	}
	rules := make([]audience.Rule, 0, len(list))
	for _, m := range list {
		match, ok1 := m["match"].(string)
		to, ok2 := m["to"].(string)
		if !ok1 || !ok2 {
			return nil, errors.New("a rule needs match and to")
		}
		rules = append(rules, audience.Rule{Match: match, To: audience.Destination(to)})
	}
	return rules, nil
}

func readLocalFile(dir sys.Dir) (local, error) {
	data, _, err := dir.Read("personal.toml")
	switch {
	case errors.Is(err, errNotExist):
		return local{}, nil
	case errors.Is(err, sys.ErrUnsafe):
		return local{}, invalidConfig("personal.toml is not a regular file")
	case errors.Is(err, sys.ErrTooLarge):
		return local{}, invalidConfig("personal.toml is too large")
	case err != nil:
		return local{}, ioError("personal.toml cannot be read")
	}
	if !utf8.Valid(data) {
		return local{}, invalidConfig("personal.toml is not UTF-8")
	}
	return decodeLocal(data)
}

// ReadLocal is the strict read of personal.toml for an audience edit or a write's inside-lock
// refresh: missing gives zero settings, and an invalid rule is an error here.
func ReadLocal(dir sys.Dir) (LocalConfig, error) {
	l, err := readLocalFile(dir)
	if err != nil {
		return LocalConfig{}, err
	}
	if l.rulesErr != nil {
		return LocalConfig{}, l.rulesErr
	}
	return l.config, nil
}

// EditAudience reads personal.toml once, through dir, and gives edit a copy of its validated rules.
// When the rules edit returns are the same rules, changed is false and nothing is to be written.
// Otherwise data is the whole new file: every other key kept, the audience rules replaced, keys in
// a stable order. The caller holds the settings lock and publishes data; comments are not kept.
func EditAudience(dir sys.Dir, edit func([]audience.Rule) ([]audience.Rule, error)) (data []byte, changed bool, err error) {
	l, err := readLocalFile(dir)
	if err != nil {
		return nil, false, err
	}
	if l.rulesErr != nil {
		return nil, false, l.rulesErr
	}
	next, err := edit(slices.Clone(l.config.Audience))
	if err != nil {
		return nil, false, err
	}
	next, err = audience.ValidateRules(next)
	if err != nil {
		return nil, false, err
	}
	if slices.Equal(next, l.config.Audience) {
		return nil, false, nil
	}
	raw := l.raw
	if raw == nil {
		raw = map[string]any{}
	}
	if len(next) == 0 {
		delete(raw, "audience")
	} else {
		tables := make([]map[string]any, len(next))
		for i, r := range next {
			tables[i] = map[string]any{"match": r.Match, "to": string(r.To)}
		}
		raw["audience"] = tables
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(raw); err != nil {
		return nil, false, ioError("personal.toml cannot be written")
	}
	return buf.Bytes(), true, nil
}

// validActor is the author rule: one non-empty line of at most 200 code points.
func validActor(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "\r\n") && utf8.ValidString(s) &&
		utf8.RuneCountInString(s) <= maxActorChars
}
