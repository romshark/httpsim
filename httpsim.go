// Package httpsim provides an HTTP simulator middleware that simplifies
// conditionally adding artificial delays and overwriting responses
// matching requests by path, headers and query parameters using glob expressions.
package httpsim

import (
	"context"
	"io"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"github.com/romshark/httpsim/internal/rand"

	"github.com/romshark/httpsim/config"
)

type Config = config.Config

// LoadConfig loads config from arbitrary reader.
func LoadConfig(src io.Reader) (*Config, error) { return config.Load(src) }

// LoadConfigFile loads config from file.
func LoadConfigFile(file string) (*Config, error) { return config.LoadFile(file) }

// CtxKey is a context.Context key type.
type CtxKey int8

// CtxKeyInfo is used by CtxInfoValue to get `CtxInfo` from `context.Context`.
const CtxKeyInfo CtxKey = 1

// CtxInfoValue gets the `CtxInfo` from ctx.
func CtxInfoValue(ctx context.Context) (info CtxInfo) {
	if v := ctx.Value(CtxKeyInfo); v != nil {
		return v.(CtxInfo)
	}
	return CtxInfo{MatchedResourceIndex: -1}
}

// CtxInfo is written to the request context after handling.
type CtxInfo struct {
	MatchedResourceIndex int
	Delay                time.Duration
	Replaced             bool
}

// RandProvider is a random values generator.
type RandProvider interface {
	// Dur returns a random duration within the given min and max range.
	Dur(min, max time.Duration) time.Duration
	// Bool returns a random boolean value.
	Bool() bool
}

// Seed is a randomness seed.
type Seed rand.Seed

// NewSeedRand creates a new random seed from `crypto/rand.Read`.
func NewSeedRand() Seed { return Seed(rand.NewSeedRand()) }

// NewSeed creates a fixed seed from a 32-byte long string.
// NewSeed panics if s isn't 32-byte long.
func NewSeed(s string) Seed { return Seed(rand.NewSeed(s)) }

var (
	defaultSeed = rand.Seed(rand.NewSeedRand())
	defaultRnd  = rand.NewSourceChaCha8(defaultSeed)
)

type defaultRand int8

// DefaultRand is the default ChaCha8 randomness provider based on a crypto-random seed.
const DefaultRand defaultRand = 1

var _ RandProvider = DefaultRand

// Seed returns the default crypto-random seed.
func (defaultRand) Seed() Seed { return Seed(defaultSeed) }

func (defaultRand) Dur(min, max time.Duration) time.Duration {
	return defaultRnd.Dur(min, max)
}
func (defaultRand) Bool() bool { return defaultRnd.Bool() }

// Sleeper is an abstract sleep. Use `DefaultSleep` for `time.Sleep`.
type Sleeper interface{ Sleep(time.Duration) }

type defaultSleep int8

// DefaultSleep is the default system sleep `time.Sleep`.
const DefaultSleep defaultSleep = 1

var _ Sleeper = DefaultSleep

func (defaultSleep) Sleep(d time.Duration) { time.Sleep(d) }

// Middleware implements the http.Handler interface.
type Middleware struct {
	rand    RandProvider
	config  atomic.Value
	sleeper Sleeper
	next    http.Handler
}

// SetConfig changes the configuration of the middleware.
// SetConfig is safe for concurrent use at runtime.
func (m *Middleware) SetConfig(c config.Config) { m.config.Store(&c) }

var _ http.Handler = new(Middleware)

// NewMiddleware creates a new middleware instance.
// Use `DefaultSleep` for sleeper
// (other implementations of Sleeper should only be used for testing purposes).
// Use `DefaultRand` for rnd if not sure.
func NewMiddleware(
	next http.Handler, c config.Config, sleeper Sleeper, rnd RandProvider,
) *Middleware {
	if sleeper == nil {
		sleeper = DefaultSleep
	}
	if rnd == nil {
		rnd = DefaultRand
	}
	m := &Middleware{rand: rnd, sleeper: sleeper, next: next}
	m.config.Store(&c)
	return m
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conf := m.config.Load().(*config.Config)
	matchedResourceIndex := Match(r, conf)
	if matchedResourceIndex != -1 {
		ctxInfo := CtxInfo{MatchedResourceIndex: matchedResourceIndex}
		ctx := r.Context()
		effect := conf.Resources[matchedResourceIndex].Effect
		if effect != nil {
			ctxInfo.Delay, ctxInfo.Replaced = m.apply(
				w, conf.Resources[matchedResourceIndex].Effect,
			)
		}
		ctx = context.WithValue(ctx, CtxKeyInfo, ctxInfo)
		r = r.WithContext(ctx)
		if ctxInfo.Replaced {
			return
		}
	}
	m.next.ServeHTTP(w, r)
}

// Match returns the index of the matched resource, otherwise returns -1.
func Match(r *http.Request, c *config.Config) int {
	for i, res := range c.Resources {
		if MatchResource(r, &res) {
			return i
		}
	}
	return -1
}

// MatchResource returns true if r matches resource c, otherwise returns false.
func MatchResource(r *http.Request, c *config.Resource) bool {
	if len(c.Methods) > 0 && !slices.Contains(c.Methods, config.HTTPMethod(r.Method)) {
		return false
	}
	if !(*config.GlobExpression)(&c.Path).Match(r.URL.Path) {
		return false
	}
	for name, values := range c.Headers {
		for header, val := range r.Header {
			if !name.Match(header) {
				continue // This header isn't mentioned in the config.
			}
			// This header is mentioned, make sure the value matches.
			if len(val) != len(values) {
				return false // Header values mismatch.
			}
			for i, val := range val {
				if !values[i].Match(val) {
					return false // Header value mismatch.
				}
			}
		}
	}
	for name, values := range c.Query {
		for parameter, val := range r.URL.Query() {
			if !name.Match(parameter) {
				continue // This parameter isn't mentioned in the config.
			}
			// This query parameter is mentioned, make sure the value matches.
			if len(val) != len(values) {
				return false // Query parameter values mismatch.
			}
			for i, val := range val {
				if !values[i].Match(val) {
					return false // Query parameter value mismatch.
				}
			}
		}
	}
	return true
}

// apply returns true if the request is handled and no further handling should be done,
// otherwise returns false.
func (m *Middleware) apply(w http.ResponseWriter, c *config.Effect) (
	delay time.Duration, replaced bool,
) {
	if c.Delay != nil {
		delay = m.rand.Dur(c.Delay.Min, c.Delay.Max)
		m.sleeper.Sleep(delay)
	}
	if c.Replace != nil {
		for header, value := range c.Replace.Headers {
			w.Header().Set(string(header), value)
		}
		if c.Replace.Body != nil {
			_, _ = w.Write([]byte(*c.Replace.Body))
		}
		w.WriteHeader(int(c.Replace.StatusCode))
		return delay, true
	}
	return delay, false
}
