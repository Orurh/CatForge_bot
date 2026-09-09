package domain

import "time"

// CatMessageReference ties a Telegram message sent by the bot to the cat that
// spoke in it. It is short-lived and can be claimed only once.
type CatMessageReference struct {
	TelegramChatID    int64
	TelegramMessageID int
	CatID             int64
	ExpiresAt         time.Time
	ClaimedAt         time.Time
}
