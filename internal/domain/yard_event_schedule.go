package domain

import "time"

// YardEventSchedule reports the same eligibility gates used by the scheduler.
type YardEventSchedule struct {
	ActiveCats int
	NextAt     time.Time
}
