package app

import (
	"context"
	"time"
)


func pickPublicChat(
	ctx context.Context,
	users UserRepository,
	userID int64,
	now time.Time,
	minInterval time.Duration,
	sourceChatID int64,
	sourceChatType string,
) (int64, error) {
	if sourceChatType != "" && sourceChatType != "private" && sourceChatID != 0 {
		return sourceChatID, nil
	}

	if users == nil {
		return 0, nil
	}
	homeID, homeType, err := users.GetHomeChat(ctx, userID)
	if err != nil || homeID == 0 || homeType == "" || homeType == "private" {
		return 0, err
	}
	ok, err := users.TryTouchHomeChatLog(ctx, userID, now, minInterval)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return homeID, nil
}