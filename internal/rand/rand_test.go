package rand_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/romshark/httpsim/internal/rand"
)

func TestNewSeed(t *testing.T) {
	s := rand.NewSeedRand()
	require.NotZero(t, s)
}

func TestSource(t *testing.T) {
	s := rand.NewSourceChaCha8(rand.NewSeed("0123456789abcdef0123456789abcdef"))
	require.False(t, s.Bool())
	require.Equal(t, time.Second, s.Dur(time.Second, time.Second))
	require.Equal(t, Dur(t, "1.684999282s"), s.Dur(time.Second, 2*time.Second))
	require.Equal(t, Dur(t, "25m7.572021725s"), s.Dur(0, 1*time.Hour))

	// Try different seed
	s = rand.NewSourceChaCha8(rand.NewSeed("fedcba9876543210fedcba9876543210"))
	require.True(t, s.Bool())
	require.Equal(t, time.Second, s.Dur(time.Second, time.Second))
	require.Equal(t, Dur(t, "1.591265866s"), s.Dur(time.Second, 2*time.Second))
	require.Equal(t, Dur(t, "47m55.022499822s"), s.Dur(0, 1*time.Hour))
}

func Dur(t *testing.T, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	require.NoError(t, err)
	return d
}

func TestNewSeedPanic(t *testing.T) {
	require.Panics(t, func() { rand.NewSeed("") })
	require.Panics(t, func() { rand.NewSeed("too short") })
	require.Panics(t, func() {
		rand.NewSeed("0123456789abcdef0123456789abcdef too long")
	})
}
