package domain

import "errors"

var (
	ErrNoCat            = errors.New("cat not found")
	ErrCatAlreadyExists = errors.New("cat already exists")
)
