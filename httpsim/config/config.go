package config

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	"unicode"

	"github.com/gobwas/glob"
	"github.com/romshark/yamagiconf"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Resources []Resource `yaml:"resources"`
}

type Resource struct {
	Methods []HTTPMethod              `yaml:"methods"`
	Path    GlobExpression            `yaml:"path"`
	Headers GlobMap[[]GlobExpression] `yaml:"headers"`
	Query   GlobMap[[]GlobExpression] `yaml:"query"`
	Effect  *Effect                   `yaml:"effect"`
}

// Headers and Query were previously implemented as slices of structs
// with Name field of type GlobExpression, but a map allows for shorter,
// nicer YAML and it's easier to use NewGlobExpression when defining a config
// programmatically. Hence, we use generic GlobMap with validation that makes
// sure the map doesn't contain duplicate keys.

type GlobMap[T any] map[GlobExpression]T

func (h GlobMap[T]) Validate() error {
	stringified := make(map[string]struct{}, len(h))
	for g := range h {
		s := fmt.Sprintf("%v", *g.glob)
		if _, ok := stringified[s]; ok {
			return fmt.Errorf("duplicate header glob: %q", s)
		}
		stringified[s] = struct{}{}
	}
	return nil
}

type HTTPMethod string

var ErrInvalidHTTPMethod = errors.New("invalid HTTP method")

// HTTPMethod must implement TextUnmarshaler for YAML decoding.
var _ encoding.TextUnmarshaler = new(HTTPMethod)

func (m *HTTPMethod) UnmarshalText(text []byte) error {
	*m = HTTPMethod(text)
	if *m == "" {
		return ErrInvalidHTTPMethod
	}
	for _, char := range string(*m) {
		if char > unicode.MaxASCII || !unicode.IsLetter(char) || unicode.IsLower(char) {
			return ErrInvalidHTTPMethod
		}
	}
	return nil
}

type Replace struct {
	StatusCode StatusCode            `yaml:"status-code"`
	Body       *string               `yaml:"body"`
	Headers    map[HeaderName]string `yaml:"headers"`
}

type Effect struct {
	Delay   *DurRange `yaml:"delay"`
	Replace *Replace  `yaml:"replace"`
}

var ErrNoEffect = errors.New("no effect")

func (e *Effect) Validate() error {
	if e == nil {
		return nil
	}
	if (e.Delay == nil || e.Delay != nil && e.Delay.Min == 0) && e.Replace == nil {
		return ErrNoEffect
	}
	return nil
}

type DurRange struct {
	Min time.Duration `yaml:"min"`
	Max time.Duration `yaml:"max"`
}

var ErrMinGreaterMax = errors.New("min greater than max")

func (r DurRange) Validate() error {
	if r.Min > r.Max {
		return ErrMinGreaterMax
	}
	return nil
}

type StatusCode int32

var ErrInvalidStatusCode = errors.New("invalid HTTP response status code")

func (c StatusCode) Validate() error {
	if http.StatusText(int(c)) == "" {
		return fmt.Errorf("%w: %d", ErrInvalidStatusCode, c)
	}
	return nil
}

type HeaderName string

var ErrInvalidHeaderName = errors.New("invalid header name")

func (n HeaderName) Validate() error {
	if n == "" {
		return ErrInvalidHeaderName
	}
	for _, char := range n {
		if !(char == '-' ||
			unicode.IsLetter(char) ||
			unicode.IsDigit(char)) ||
			char > unicode.MaxASCII {
			return ErrInvalidHeaderName
		}
	}
	return nil
}

type GlobExpression struct {
	// glob is a pointer to make the struct comparable
	// and allow it to be used as map key.
	glob *glob.Glob
}

func (e GlobExpression) String() string { return fmt.Sprintf("%v", e.glob) }

func NewGlobExpression(expression string) (GlobExpression, error) {
	g, err := glob.Compile(expression)
	if err != nil {
		return GlobExpression{}, err
	}
	return GlobExpression{glob: &g}, nil
}

// GlobExpression must implement TextUnmarshaler for YAML decoding.
var (
	_ encoding.TextUnmarshaler = new(GlobExpression)
	_ glob.Glob                = new(GlobExpression)
)

func (g *GlobExpression) UnmarshalText(text []byte) (err error) {
	c, err := glob.Compile(string(text))
	if err != nil {
		return err
	}
	g.glob = &c
	return nil
}

func (g *GlobExpression) Match(s string) bool {
	if g.glob == nil {
		return true
	}
	return (*g.glob).Match(s)
}

// Validate returns an error if c is invalid, otherwise returns nil.
func Validate(c Config) error { return yamagiconf.Validate(c) }

// Load loads config from arbitrary reader.
func Load(src io.Reader) (*Config, error) {
	var c Config
	// Use standard YAML decoder but utilize yamagiconf validation.
	d := yaml.NewDecoder(src)
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("decoding YAML: %w", err)
	}
	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("validating: %w", err)
	}
	return &c, nil
}

// LoadFile loads config from file.
func LoadFile(file string) (*Config, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	return Load(f)
}
