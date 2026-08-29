package domain

import "errors"

var (
	ErrNoCat                = errors.New("cat not found")
	ErrCatAlreadyExists     = errors.New("cat already exists")
	ErrConcurrentUpdate     = errors.New("cat state changed concurrently")
	ErrItemNotOwned         = errors.New("item not owned")
	ErrItemMaxLevel         = errors.New("item is already at maximum level")
	ErrNotEnoughFragments   = errors.New("not enough item fragments")
	ErrNotEnoughCoins       = errors.New("not enough coins")
	ErrAIRateLimited        = errors.New("AI request rate limited")
	ErrNoYard               = errors.New("yard not found")
	ErrNotYardMember        = errors.New("cat is not a yard member")
	ErrYardEventUnavailable = errors.New("yard event unavailable")
	ErrInvalidYardChoice    = errors.New("invalid yard event choice")
)
