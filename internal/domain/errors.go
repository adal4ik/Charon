package domain

import "errors"

var (
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrAccountNotFound   = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrSameAccount       = errors.New("transfer to same account")
)
