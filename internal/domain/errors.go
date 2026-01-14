package domain

import "errors"

var (
	ErrNoCat            = errors.New("cat not found")
	ErrCatAlreadyExists = errors.New("cat already exists")
)

var (
	ErrArenaRerollCooldown  = errors.New("arena: reroll cooldown")
	ErrArenaNotEnoughEnergy = errors.New("arena: not enough energy")
)