package randx

// RNG is a minimal randomness interface (test-friendly).
type RNG interface{ Intn(n int) int }
