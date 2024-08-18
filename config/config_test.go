package config_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/romshark/yamagiconf"
	"github.com/stretchr/testify/require"

	"github.com/romshark/httpsim/config"
)

func TestValidateConfigType(t *testing.T) {
	err := yamagiconf.ValidateType[config.Config]()
	require.NoError(t, err)
}

func TestHTTPMethod(t *testing.T) {
	f := func(input string, fn require.ErrorAssertionFunc) {
		t.Helper()
		var m config.HTTPMethod
		err := m.UnmarshalText([]byte(input))
		require.Equal(t, input, string(m))
		fn(t, err)
	}

	f(http.MethodGet, require.NoError)
	f(http.MethodHead, require.NoError)
	f(http.MethodPost, require.NoError)
	f(http.MethodPut, require.NoError)
	f(http.MethodPatch, require.NoError)
	f(http.MethodDelete, require.NoError)
	f(http.MethodConnect, require.NoError)
	f(http.MethodOptions, require.NoError)
	f(http.MethodTrace, require.NoError)
	f("CUSTOM", require.NoError)

	f("invalid", require.Error)
	f("", require.Error)
	f(" ", require.Error)
	f("INVALID!", require.Error)
	f("IN-VALID", require.Error)
	f("IN_VALID", require.Error)
	f("IN\nVALID", require.Error)
}

func TestHTTPStatusCode(t *testing.T) {
	f := func(input int, fn require.ErrorAssertionFunc) {
		t.Helper()
		err := config.StatusCode(input).Validate()
		fn(t, err)
	}

	f(http.StatusContinue, require.NoError)
	f(http.StatusSwitchingProtocols, require.NoError)
	f(http.StatusProcessing, require.NoError)
	f(http.StatusEarlyHints, require.NoError)
	f(http.StatusOK, require.NoError)
	f(http.StatusCreated, require.NoError)
	f(http.StatusAccepted, require.NoError)
	f(http.StatusNonAuthoritativeInfo, require.NoError)
	f(http.StatusNoContent, require.NoError)
	f(http.StatusResetContent, require.NoError)
	f(http.StatusPartialContent, require.NoError)
	f(http.StatusMultiStatus, require.NoError)
	f(http.StatusAlreadyReported, require.NoError)
	f(http.StatusIMUsed, require.NoError)
	f(http.StatusMultipleChoices, require.NoError)
	f(http.StatusMovedPermanently, require.NoError)
	f(http.StatusFound, require.NoError)
	f(http.StatusSeeOther, require.NoError)
	f(http.StatusNotModified, require.NoError)
	f(http.StatusUseProxy, require.NoError)
	f(http.StatusTemporaryRedirect, require.NoError)
	f(http.StatusPermanentRedirect, require.NoError)
	f(http.StatusBadRequest, require.NoError)
	f(http.StatusUnauthorized, require.NoError)
	f(http.StatusPaymentRequired, require.NoError)
	f(http.StatusForbidden, require.NoError)
	f(http.StatusNotFound, require.NoError)
	f(http.StatusMethodNotAllowed, require.NoError)
	f(http.StatusNotAcceptable, require.NoError)
	f(http.StatusProxyAuthRequired, require.NoError)
	f(http.StatusRequestTimeout, require.NoError)
	f(http.StatusConflict, require.NoError)
	f(http.StatusGone, require.NoError)
	f(http.StatusLengthRequired, require.NoError)
	f(http.StatusPreconditionFailed, require.NoError)
	f(http.StatusRequestEntityTooLarge, require.NoError)
	f(http.StatusRequestURITooLong, require.NoError)
	f(http.StatusUnsupportedMediaType, require.NoError)
	f(http.StatusRequestedRangeNotSatisfiable, require.NoError)
	f(http.StatusExpectationFailed, require.NoError)
	f(http.StatusTeapot, require.NoError)
	f(http.StatusMisdirectedRequest, require.NoError)
	f(http.StatusUnprocessableEntity, require.NoError)
	f(http.StatusLocked, require.NoError)
	f(http.StatusFailedDependency, require.NoError)
	f(http.StatusTooEarly, require.NoError)
	f(http.StatusUpgradeRequired, require.NoError)
	f(http.StatusPreconditionRequired, require.NoError)
	f(http.StatusTooManyRequests, require.NoError)
	f(http.StatusRequestHeaderFieldsTooLarge, require.NoError)
	f(http.StatusUnavailableForLegalReasons, require.NoError)
	f(http.StatusInternalServerError, require.NoError)
	f(http.StatusNotImplemented, require.NoError)
	f(http.StatusBadGateway, require.NoError)
	f(http.StatusServiceUnavailable, require.NoError)
	f(http.StatusGatewayTimeout, require.NoError)
	f(http.StatusHTTPVersionNotSupported, require.NoError)
	f(http.StatusVariantAlsoNegotiates, require.NoError)
	f(http.StatusInsufficientStorage, require.NoError)
	f(http.StatusLoopDetected, require.NoError)
	f(http.StatusNotExtended, require.NoError)
	f(http.StatusNetworkAuthenticationRequired, require.NoError)

	f(306, require.Error) // RFC 9110, 15.4.7 (Unused)
	f(-400, require.Error)
	f(0, require.Error)
	f(1000, require.Error)
}

func TestHeaderName(t *testing.T) {
	f := func(input string, fn require.ErrorAssertionFunc) {
		t.Helper()
		err := config.HeaderName(input).Validate()
		fn(t, err)
	}

	f("Content-Type", require.NoError)
	f("X-Custom", require.NoError)

	f("", require.Error)
	f(" ", require.Error)
}

func TestDurRange(t *testing.T) {
	require.NoError(t, config.DurRange{Min: 0, Max: 0}.Validate())
	require.NoError(t, config.DurRange{Min: 1, Max: 1}.Validate())
	require.NoError(t, config.DurRange{Min: 10, Max: 11}.Validate())
	require.NoError(t, config.DurRange{Min: -10, Max: 1}.Validate())

	require.Error(t, config.DurRange{Min: 1, Max: 0}.Validate())
}

func TestLoadFile(t *testing.T) {
	p := TmpFile(t, `
resources:
  - path: /specific
    methods: [DELETE]
    effect:
      replace:
        status-code: 404
        body: "Specific resource not found"
        headers:
          Content-Type: text/plain
          X-Custom: custom
  - path: /* # anything behind root
    methods: [GET, POST]
    headers:
      "Content-Type": ["application/javascript"]
    query:
      "param": ["щы"]
    effect:
      delay:
        min: 200ms
        max: 10s
`)
	c, err := config.LoadFile(p)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestLoadFileErrValidation(t *testing.T) {
	p := TmpFile(t, `
resources:
  - path: /specific
    methods: [DELETE]
    effect:
      replace:
        status-code: -400
`)
	c, err := config.LoadFile(p)
	require.ErrorIs(t, err, config.ErrInvalidStatusCode)
	require.Nil(t, c)
}

func TestLoadFileErrValidationNoEffect(t *testing.T) {
	p := TmpFile(t, `
resources:
  - path: /specific
    effect:
      delay:
        min: 0s
`)
	c, err := config.LoadFile(p)
	require.ErrorIs(t, err, config.ErrNoEffect)
	require.Nil(t, c)
}

func TestLoadFileNoEffect(t *testing.T) {
	p := TmpFile(t, `
resources:
  - path: /a
    effect:
  - path: /b
    effect: null
`)
	c, err := config.LoadFile(p)
	require.NoError(t, err)
	require.Equal(t, &config.Config{
		Resources: []config.Resource{
			{Path: NewGlobExpression(t, "/a")},
			{Path: NewGlobExpression(t, "/b")},
		},
	}, c)
}

func TestLoadFileErrNotExist(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test_config.yaml")
	c, err := config.LoadFile(p)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.True(t, strings.HasPrefix(err.Error(), "opening file: "))
	require.Nil(t, c)
}

func TestLoadFileErrInvalidYAML(t *testing.T) {
	p := TmpFile(t, `resources: "invalid"`)
	c, err := config.LoadFile(p)
	require.True(t, strings.HasPrefix(err.Error(), "decoding YAML: "))
	require.Nil(t, c)
}

func TestNewGlobExpression(t *testing.T) {
	g, err := config.NewGlobExpression("/foo/bar/*")
	require.NoError(t, err)
	require.True(t, g.Match("/foo/bar/"))
	require.True(t, g.Match("/foo/bar/bazz"))
	require.False(t, g.Match("/bar/foo"))

	{
		const invalidUTF8 = "\xC3\x28"
		require.False(t, utf8.ValidString(invalidUTF8))

		_, err := config.NewGlobExpression(invalidUTF8)
		require.Error(t, err)
		require.Equal(t, "could not read rune", err.Error())
	}
}

func TestGlobMatchUninitialized(t *testing.T) {
	var uninitialized config.GlobExpression
	require.True(t, uninitialized.Match("test"))
}

func TmpFile(t *testing.T, contents string) (tmpFilePath string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test_config.yaml")
	err := os.WriteFile(p, []byte(contents), 0o777)
	require.NoError(t, err)
	return p
}

func NewGlobExpression(t *testing.T, expr string) config.GlobExpression {
	t.Helper()
	e, err := config.NewGlobExpression(expr)
	require.NoError(t, err)
	return e
}
