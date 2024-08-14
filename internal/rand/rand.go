// Package rand is a helper package wrapping std lib functions.
package rand

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/rand/v2"
	"time"
)

// Seed is a chacha8 randomness seed.
type Seed [32]byte

// NewSeed creates a certain seed from 32-byte long string.
// NewSeed panics if s isn't 32-byte long.
func NewSeed(s string) Seed {
	if len(s) != 32 {
		panic(fmt.Errorf("seed must be a 32-byte long string"))
	}
	var seed [32]byte
	copy(seed[:], s)
	return seed
}

// NewSeedRand creates a new random seed using `crypto/rand.Read`.
func NewSeedRand() Seed {
	var seed [32]byte
	_, err := cryptorand.Read(seed[:])
	if err != nil {
		panic(err)
	}
	return Seed(seed)
}

// Source is a randomness source.
type Source struct{ r *rand.Rand }

// NewSourceChaCha8 returns a chacha8 based randomness source.
func NewSourceChaCha8(seed Seed) Source {
	return Source{r: rand.New(rand.NewChaCha8(seed))}
}

// Dur returns a random duration within the given min and max range.
func (s Source) Dur(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	delta := max - min
	return min + time.Duration(s.r.Int64N(int64(delta)))
}

// Bool returns a random boolean value.
func (s Source) Bool() bool { return s.r.IntN(2) == 1 }
