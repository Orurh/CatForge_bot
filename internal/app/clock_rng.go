package app

import (
	"math/rand"
	"time"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type MathRNG struct{}

func (MathRNG) Intn(n int) int { return rand.Intn(n) }
