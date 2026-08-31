package service

import (
	"context"

	"github.com/adal4ik/Charon/internal/domain"
)

// AccountRepository defines the persistence operations required by Service.
type AccountRepository interface {
	Deposit(ctx context.Context, accountID int64, amount int64) (domain.Account, error)
	Transfer(ctx context.Context, fromAccountID int64, toAccountID int64, amount int64) error
}

type Service struct {
	repository AccountRepository
}

func New(repository AccountRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Deposit(
	ctx context.Context,
	accountID int64,
	amount int64,
) (domain.Account, error) {
	if amount <= 0 {
		return domain.Account{}, domain.ErrInvalidAmount
	}

	return s.repository.Deposit(ctx, accountID, amount)
}

func (s *Service) Transfer(
	ctx context.Context,
	fromAccountID int64,
	toAccountID int64,
	amount int64,
) error {
	if amount <= 0 {
		return domain.ErrInvalidAmount
	}
	if fromAccountID == toAccountID {
		return domain.ErrSameAccount
	}

	return s.repository.Transfer(ctx, fromAccountID, toAccountID, amount)
}
