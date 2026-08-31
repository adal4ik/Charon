package repository_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/adal4ik/Charon/internal/domain"
	"github.com/adal4ik/Charon/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryTransferConcurrent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	setupCtx, setupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer setupCancel()

	pool, err := pgxpool.New(setupCtx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(setupCtx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	accountRepository := repository.NewPostgresRepository(pool)
	createdAccountIDs := make([]int64, 0, 3)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		for _, accountID := range createdAccountIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM accounts WHERE id = $1", accountID); err != nil {
				t.Errorf("delete test account %d: %v", accountID, err)
			}
		}
	})

	createAccount := func() domain.Account {
		t.Helper()

		account, err := accountRepository.CreateAccount(setupCtx)
		if err != nil {
			t.Fatalf("create test account (are migrations applied?): %v", err)
		}
		createdAccountIDs = append(createdAccountIDs, account.ID)

		return account
	}

	sender := createAccount()
	firstReceiver := createAccount()
	secondReceiver := createAccount()

	if _, err := accountRepository.Deposit(setupCtx, sender.ID, 100); err != nil {
		t.Fatalf("deposit sender balance: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)

	runTransfer := func(receiverID int64) {
		go func() {
			transferCtx, transferCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer transferCancel()

			ready.Done()
			<-start

			results <- accountRepository.Transfer(transferCtx, sender.ID, receiverID, 60)
		}()
	}

	runTransfer(firstReceiver.ID)
	runTransfer(secondReceiver.ID)
	ready.Wait()
	close(start)

	successCount := 0
	insufficientFundsCount := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, domain.ErrInsufficientFunds):
			insufficientFundsCount++
		default:
			t.Errorf("Transfer() returned unexpected error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("successful transfers = %d, want 1", successCount)
	}
	if insufficientFundsCount != 1 {
		t.Errorf("insufficient funds errors = %d, want 1", insufficientFundsCount)
	}

	getAccount := func(accountID int64) domain.Account {
		t.Helper()

		account, err := accountRepository.GetAccount(setupCtx, accountID)
		if err != nil {
			t.Fatalf("get account %d: %v", accountID, err)
		}

		return account
	}

	sender = getAccount(sender.ID)
	firstReceiver = getAccount(firstReceiver.ID)
	secondReceiver = getAccount(secondReceiver.ID)

	if sender.Balance != 40 {
		t.Errorf("sender balance = %d, want 40", sender.Balance)
	}
	if firstReceiver.Balance+secondReceiver.Balance != 60 {
		t.Errorf(
			"receiver balance sum = %d, want 60",
			firstReceiver.Balance+secondReceiver.Balance,
		)
	}
	receiverBalancesMatch :=
		(firstReceiver.Balance == 60 && secondReceiver.Balance == 0) ||
			(firstReceiver.Balance == 0 && secondReceiver.Balance == 60)
	if !receiverBalancesMatch {
		t.Errorf(
			"receiver balances = (%d, %d), want (60, 0) or (0, 60)",
			firstReceiver.Balance,
			secondReceiver.Balance,
		)
	}

	for _, account := range []domain.Account{sender, firstReceiver, secondReceiver} {
		if account.Balance < 0 {
			t.Errorf("account %d has negative balance %d", account.ID, account.Balance)
		}
	}
}
