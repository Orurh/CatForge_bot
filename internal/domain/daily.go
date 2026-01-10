package domain

import "time"

type DailyClaimOutcome int

const (
	DailyClaimOK DailyClaimOutcome = iota
	DailyClaimAlreadyClaimed
)

// DailyState is stored in users table.
// LastClaimDay is a "date" concept (we treat it as UTC midnight).
type DailyState struct {
	LastClaimDay time.Time
	Streak       int
}

type DailyView struct {
	CanClaim bool
	Streak   int
	NextAt   time.Time
}

type DailyClaimResult struct {
	Outcome    DailyClaimOutcome
	Streak     int
	XPGain     int64
	EnergyGain int
	NextAt     time.Time
}

func DailyDay(now time.Time) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func NextDailyAt(now time.Time) time.Time {
	day := DailyDay(now)
	return day.Add(24 * time.Hour)
}

func CanClaimDaily(lastClaimDay, now time.Time) bool {
	if lastClaimDay.IsZero() {
		return true
	}
	return !DailyDay(lastClaimDay).Equal(DailyDay(now))
}

func MakeDailyView(st DailyState, now time.Time) DailyView {
	return DailyView{
		CanClaim: CanClaimDaily(st.LastClaimDay, now),
		Streak:   st.Streak,
		NextAt:   NextDailyAt(now),
	}
}
