package app

import (
	"math/rand"
	"time"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

var gameTimeLocation = time.FixedZone("Europe/Moscow", 3*60*60)

// GameTime keeps daily and weekly gameplay boundaries stable even when the
// container itself runs in UTC.
func GameTime(value time.Time) time.Time { return value.In(gameTimeLocation) }

type MathRNG struct{}

func (MathRNG) Intn(n int) int { return rand.Intn(n) }
