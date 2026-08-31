package domain

import "time"

// Account is a virtual account whose balance is stored in minimal monetary units.
type Account struct {
	ID        int64
	Balance   int64
	CreatedAt time.Time
}
