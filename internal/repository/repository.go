package repository

import (
	"context"

	"github.com/adal4ik/Charon/internal/domain"
)

// AccountRepository defines persistence operations required by the account service.
type AccountRepository interface {
	CreateAccount(ctx context.Context) (domain.Account, error)
	GetAccount(ctx context.Context, accountID int64) (domain.Account, error)
	Deposit(ctx context.Context, accountID, amount int64) (domain.Account, error)
	Transfer(ctx context.Context, fromAccountID, toAccountID, amount int64) error
}
