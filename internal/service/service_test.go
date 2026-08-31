package service

import (
	"context"
	"errors"
	"testing"

	"github.com/adal4ik/Charon/internal/domain"
)

type fakeAccountRepository struct {
	transferCalls int
	fromAccountID int64
	toAccountID   int64
	amount        int64
	transferErr   error
}

func (f *fakeAccountRepository) Deposit(
	context.Context,
	int64,
	int64,
) (domain.Account, error) {
	panic("unexpected call to Deposit")
}

func (f *fakeAccountRepository) Transfer(
	_ context.Context,
	fromAccountID int64,
	toAccountID int64,
	amount int64,
) error {
	f.transferCalls++
	f.fromAccountID = fromAccountID
	f.toAccountID = toAccountID
	f.amount = amount

	return f.transferErr
}

func TestServiceTransferSuccess(t *testing.T) {
	repository := &fakeAccountRepository{}
	service := New(repository)

	err := service.Transfer(context.Background(), 10, 20, 60)

	if err != nil {
		t.Fatalf("Transfer() error = %v, want nil", err)
	}
	if repository.transferCalls != 1 {
		t.Fatalf("repository.Transfer() calls = %d, want 1", repository.transferCalls)
	}
	if repository.fromAccountID != 10 {
		t.Errorf("repository fromAccountID = %d, want 10", repository.fromAccountID)
	}
	if repository.toAccountID != 20 {
		t.Errorf("repository toAccountID = %d, want 20", repository.toAccountID)
	}
	if repository.amount != 60 {
		t.Errorf("repository amount = %d, want 60", repository.amount)
	}
}

func TestServiceTransferInvalidAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
	}{
		{name: "zero", amount: 0},
		{name: "negative", amount: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeAccountRepository{}
			service := New(repository)

			err := service.Transfer(context.Background(), 10, 20, tt.amount)

			if !errors.Is(err, domain.ErrInvalidAmount) {
				t.Fatalf("Transfer() error = %v, want %v", err, domain.ErrInvalidAmount)
			}
			if repository.transferCalls != 0 {
				t.Fatalf("repository.Transfer() calls = %d, want 0", repository.transferCalls)
			}
		})
	}
}

func TestServiceTransferSameAccount(t *testing.T) {
	repository := &fakeAccountRepository{}
	service := New(repository)

	err := service.Transfer(context.Background(), 10, 10, 60)

	if !errors.Is(err, domain.ErrSameAccount) {
		t.Fatalf("Transfer() error = %v, want %v", err, domain.ErrSameAccount)
	}
	if repository.transferCalls != 0 {
		t.Fatalf("repository.Transfer() calls = %d, want 0", repository.transferCalls)
	}
}

func TestServiceTransferInsufficientFunds(t *testing.T) {
	repository := &fakeAccountRepository{transferErr: domain.ErrInsufficientFunds}
	service := New(repository)

	err := service.Transfer(context.Background(), 10, 20, 60)

	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("Transfer() error = %v, want %v", err, domain.ErrInsufficientFunds)
	}
	if repository.transferCalls != 1 {
		t.Fatalf("repository.Transfer() calls = %d, want 1", repository.transferCalls)
	}
}
