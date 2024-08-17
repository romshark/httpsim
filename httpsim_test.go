package httpsim_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/romshark/httpsim"
	"github.com/romshark/httpsim/config"
	"github.com/romshark/httpsim/internal/rand"
)

func TestCtxInfoValue(t *testing.T) {
	ctx := context.Background()
	v := httpsim.CtxInfoValue(ctx)
	require.Equal(t, httpsim.CtxInfo{
		MatchedResourceIndex: -1,
	}, v)

	ctx = context.WithValue(ctx, httpsim.CtxKeyInfo, httpsim.CtxInfo{
		MatchedResourceIndex: 42,
	})
	v = httpsim.CtxInfoValue(ctx)
	require.Equal(t, httpsim.CtxInfo{
		MatchedResourceIndex: 42,
	}, v)
}

func TestDefaultRand(t *testing.T) {
	require.NotZero(t, httpsim.DefaultRand.Seed())
	var p httpsim.RandProvider = httpsim.DefaultRand
	require.Equal(t, time.Second, p.Dur(time.Second, time.Second))
	// We don't care about the result values, just make sure we can call it.
	_ = p.Bool()
}

func TestDefaultSleep(t *testing.T) {
	var s httpsim.Sleeper = httpsim.DefaultSleep
	s.Sleep(0) // We just want to make sure we can call it.
}

func TestNewSeedRand(t *testing.T) {
	s := httpsim.NewSeedRand()
	require.NotZero(t, s)
}

func TestMatchResource(t *testing.T) {
	f := func(resource config.Resource, r *http.Request, expect bool) {
		t.Helper()
		actual := httpsim.MatchResource(r, &resource)
		require.Equal(t, expect, actual)
	}

	f( // Path matches.
		config.Resource{},
		NewRequest(t, http.MethodPost, "https://host.io/x", http.NoBody),
		true,
	)
	f( // Concrete path matches.
		config.Resource{
			Path: NewGlobExpression(t, "/foo/bar"),
		},
		NewRequest(t, http.MethodPost, "https://host.io/foo/bar", http.NoBody),
		true,
	)
	f( // Method and path match
		config.Resource{
			Methods: []config.HTTPMethod{http.MethodPost, http.MethodDelete},
		},
		NewRequest(t, http.MethodDelete, "https://host.io/x", http.NoBody),
		true,
	)
	f( // Header matches.
		config.Resource{
			Headers: map[config.GlobExpression][]config.GlobExpression{
				NewGlobExpression(t, "Content-Type"): {
					NewGlobExpression(t, "application/*"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			r.Header.Set("Content-Type", "application/json")
			return r
		}(),
		true,
	)
	f( // Ignore headers.
		config.Resource{
			Headers: map[config.GlobExpression][]config.GlobExpression{
				NewGlobExpression(t, "Content-Type"): {
					NewGlobExpression(t, "application/*"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			r.Header.Set("X-Custom", "ignore this header")
			return r
		}(),
		true,
	)
	f( // Ignore query parameter.
		config.Resource{
			Query: config.GlobMap[[]config.GlobExpression]{
				NewGlobExpression(t, "foo"): []config.GlobExpression{
					NewGlobExpression(t, "bar"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			q := r.URL.Query()
			q.Set("bazz", "mazz")
			r.URL.RawQuery = q.Encode()
			return r
		}(),
		true,
	)

	/*** No match ***/

	f( // Path mismatch.
		config.Resource{
			Methods: []config.HTTPMethod{http.MethodGet},
			Path:    NewGlobExpression(t, "/x*"),
		},
		NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody),
		false,
	)
	f( // Method mismatch.
		config.Resource{
			Methods: []config.HTTPMethod{http.MethodGet, http.MethodPost},
		},
		NewRequest(t, http.MethodDelete, "https://host.io/", http.NoBody),
		false,
	)
	f( // Header value mismatch.
		config.Resource{
			Headers: map[config.GlobExpression][]config.GlobExpression{
				NewGlobExpression(t, "Content-Type"): {
					NewGlobExpression(t, "application/json"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			r.Header.Set("Content-Type", "application/javascript")
			return r
		}(),
		false,
	)
	f( // Header number of values mismatch.
		config.Resource{
			Headers: map[config.GlobExpression][]config.GlobExpression{
				NewGlobExpression(t, "X-Custom"): {
					NewGlobExpression(t, "foo"),
					NewGlobExpression(t, "bar"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			r.Header.Set("X-Custom", "foo,bar,baz")
			return r
		}(),
		false,
	)
	f( // Query parameter "foo" value mismatch.
		config.Resource{
			Query: config.GlobMap[[]config.GlobExpression]{
				NewGlobExpression(t, "foo"): []config.GlobExpression{
					NewGlobExpression(t, "bar"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			q := r.URL.Query()
			q.Set("foo", "mazz")
			r.URL.RawQuery = q.Encode()
			return r
		}(),
		false,
	)
	f( // Query number of argument values mismatch.
		config.Resource{
			Query: config.GlobMap[[]config.GlobExpression]{
				NewGlobExpression(t, "multivalparam"): []config.GlobExpression{
					NewGlobExpression(t, "foo"),
					NewGlobExpression(t, "bar"),
				},
			},
		},
		func() *http.Request {
			r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
			q := r.URL.Query()
			q.Set("multivalparam", "foo,bar,baz")
			r.URL.RawQuery = q.Encode()
			return r
		}(),
		false,
	)
}

func TestMatch(t *testing.T) {
	c := &config.Config{
		Resources: []config.Resource{
			{Methods: []config.HTTPMethod{http.MethodGet}},
			{Methods: []config.HTTPMethod{http.MethodPost}},
		},
	}
	{
		r := NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody)
		i := httpsim.Match(r, c)
		require.Equal(t, 0, i)
	}
	{
		r := NewRequest(t, http.MethodPost, "https://host.io/", http.NoBody)
		i := httpsim.Match(r, c)
		require.Equal(t, 1, i)
	}
	{
		r := NewRequest(t, http.MethodDelete, "https://host.io/", http.NoBody)
		i := httpsim.Match(r, c)
		require.Equal(t, -1, i)
	}
}

func NewGlobExpression(t *testing.T, expression string) config.GlobExpression {
	t.Helper()
	g, err := config.NewGlobExpression(expression)
	require.NoError(t, err)
	return g
}

func NewRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	r, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	return r
}

func NewURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	require.NoError(t, err)
	return u
}

func NewDuration(t *testing.T, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	require.NoError(t, err)
	return d
}

type MockSleep struct{ Cumulative time.Duration }

func (s *MockSleep) Sleep(d time.Duration) { s.Cumulative += d }

var _ httpsim.Sleeper = new(MockSleep)

func TestHandleDur(t *testing.T) {
	expectedDelay := NewDuration(t, "1.394636475s")
	conf := config.Config{
		Resources: []config.Resource{
			{Effect: &config.Effect{
				Delay: &config.DurRange{Min: time.Second, Max: 2 * time.Second},
			}},
		},
	}
	nextInvoked := false
	mockSleep, s := NewSimulator(
		t, conf,
		func(w http.ResponseWriter, r *http.Request) {
			nextInvoked = true
			info := httpsim.CtxInfoValue(r.Context())
			require.Equal(t, httpsim.CtxInfo{
				MatchedResourceIndex: 0,
				Delay:                expectedDelay,
			}, info)
		},
	)
	rec := httptest.NewRecorder()
	req := NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody)

	s.ServeHTTP(rec, req)
	require.True(t, nextInvoked)

	require.Equal(t, expectedDelay, mockSleep.Cumulative)
	require.Len(t, (rec.Header()), 0)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, rec.Body.String())
}

func TestHandleReplace(t *testing.T) {
	replacedBody := "replaced body"
	conf := config.Config{
		Resources: []config.Resource{
			{Effect: &config.Effect{
				Delay: &config.DurRange{Min: time.Second, Max: 2 * time.Second},
				Replace: &config.Replace{
					StatusCode: http.StatusInternalServerError,
					Body:       &replacedBody,
					Headers: map[config.HeaderName]string{
						"X-CustomAdd":     "added",
						"X-CustomReplace": "replaced",
					},
				},
			}},
		},
	}
	nextInvoked := false
	mockSleep, s := NewSimulator(t, conf, func(w http.ResponseWriter, r *http.Request) {
		nextInvoked = true
	})
	rec := httptest.NewRecorder()
	req := NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody)

	s.ServeHTTP(rec, req)
	require.False(t, nextInvoked)

	require.Equal(t, NewDuration(t, "1.394636475s"), mockSleep.Cumulative)
	require.Len(t, (rec.Header()), 3)
	require.Equal(t, rec.Header().Get("Content-Type"), "text/plain; charset=utf-8")
	require.Equal(t, rec.Header().Get("X-CustomAdd"), "added")
	require.Equal(t, rec.Header().Get("X-CustomReplace"), "replaced")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "replaced body", rec.Body.String())
}

func TestHandleReplaceStatusCodeOnly(t *testing.T) {
	conf := config.Config{
		Resources: []config.Resource{
			{Effect: &config.Effect{
				Replace: &config.Replace{
					StatusCode: http.StatusNoContent,
				},
			}},
		},
	}
	nextInvoked := false
	mockSleep, s := NewSimulator(t, conf, func(w http.ResponseWriter, r *http.Request) {
		nextInvoked = true
	})
	rec := httptest.NewRecorder()
	req := NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody)

	s.ServeHTTP(rec, req)
	require.False(t, nextInvoked)

	require.Zero(t, mockSleep.Cumulative)
	require.Len(t, (rec.Header()), 0)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Zero(t, rec.Body.String())
}

func TestHandleNoMatch(t *testing.T) {
	replacedBody := "replaced body"
	conf := config.Config{
		Resources: []config.Resource{
			{
				Methods: []config.HTTPMethod{http.MethodDelete},
				Effect: &config.Effect{
					Delay: &config.DurRange{Min: time.Second, Max: 2 * time.Second},
					Replace: &config.Replace{
						StatusCode: http.StatusInternalServerError,
						Body:       &replacedBody,
						Headers: map[config.HeaderName]string{
							"X-CustomAdd":     "added",
							"X-CustomReplace": "replaced",
						},
					},
				},
			},
		},
	}
	nextInvoked := false
	mockSleep, s := NewSimulator(t, conf, func(w http.ResponseWriter, r *http.Request) {
		nextInvoked = true
		info := httpsim.CtxInfoValue(r.Context())
		require.Equal(t, httpsim.CtxInfo{
			MatchedResourceIndex: -1,
		}, info)
	})
	rec := httptest.NewRecorder()
	req := NewRequest(t, http.MethodGet, "https://host.io/", http.NoBody)
	req.Header.Set("X-CustomReplace", "old value")

	s.ServeHTTP(rec, req)
	require.True(t, nextInvoked)

	require.Zero(t, mockSleep.Cumulative)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, rec.Body.String())
}

func TestNewSeedPanic(t *testing.T) {
	require.Panics(t, func() { httpsim.NewSeed("") })
	require.Panics(t, func() { httpsim.NewSeed("too short") })
	require.Panics(t, func() {
		httpsim.NewSeed("0123456789abcdef0123456789abcdef too long")
	})
}

func NewSimulator(
	t *testing.T, conf config.Config,
	handlerFunc http.HandlerFunc,
) (*MockSleep, *httpsim.Middleware) {
	t.Helper()
	require.NoError(t, config.Validate(conf))
	mockSleep := new(MockSleep)
	seed := httpsim.NewSeed("fedcba9876543210fedcba9876543210")
	rnd := rand.NewSourceChaCha8(rand.Seed(seed))
	s := httpsim.NewMiddleware(handlerFunc, conf, mockSleep, rnd)
	return mockSleep, s
}
