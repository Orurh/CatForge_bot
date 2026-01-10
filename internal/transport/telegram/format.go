package telegram

import (
	"fmt"
	"strconv"
)

func itoa(v int) string     { return strconv.Itoa(v) }
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// fmtMul formats percent (0..100) as a multiplier string (e.g. 52 -> "x0.52").
func fmtMul(percent int) string {
	switch {
	case percent <= 0:
		return "x0.00"
	case percent >= 100:
		return "x1.00"
	default:
		return fmt.Sprintf("x%.2f", float64(percent)/100.0)
	}
}