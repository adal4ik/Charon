package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/adal4ik/Charon/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rollbackTimeout = 5 * time.Second

// PostgresRepository stores accounts in PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateAccount(ctx context.Context) (domain.Account, error) {
	const query = `
		INSERT INTO accounts DEFAULT VALUES
		RETURNING id, balance, created_at`

	var account domain.Account
	err := r.pool.QueryRow(ctx, query).Scan(
		&account.ID,
		&account.Balance,
		&account.CreatedAt,
	)
	if err != nil {
		return domain.Account{}, fmt.Errorf("create account: %w", err)
	}

	return account, nil
}

func (r *PostgresRepository) GetAccount(ctx context.Context, accountID int64) (domain.Account, error) {
	const query = `
		SELECT id, balance, created_at
		FROM accounts
		WHERE id = $1`

	var account domain.Account
	err := r.pool.QueryRow(ctx, query, accountID).Scan(
		&account.ID,
		&account.Balance,
		&account.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("get account: %w", err)
	}

	return account, nil
}

func (r *PostgresRepository) Deposit(
	ctx context.Context,
	accountID int64,
	amount int64,
) (domain.Account, error) {
	const query = `
		UPDATE accounts
		SET balance = balance + $2
		WHERE id = $1
		RETURNING id, balance, created_at`

	var account domain.Account
	err := r.pool.QueryRow(ctx, query, accountID, amount).Scan(
		&account.ID,
		&account.Balance,
		&account.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("deposit to account: %w", err)
	}

	return account, nil
}

func (r *PostgresRepository) Transfer(
	ctx context.Context,
	fromAccountID int64,
	toAccountID int64,
	amount int64,
) (resultErr error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transfer transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}

		rollbackCtx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
		defer cancel()

		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			rollbackErr := fmt.Errorf("rollback transfer transaction: %w", err)
			if resultErr == nil {
				resultErr = rollbackErr
				return
			}
			resultErr = errors.Join(resultErr, rollbackErr)
		}
	}()

	firstAccountID, secondAccountID := fromAccountID, toAccountID
	if firstAccountID > secondAccountID {
		firstAccountID, secondAccountID = secondAccountID, firstAccountID
	}

	const lockQuery = `
		SELECT id, balance
		FROM accounts
		WHERE id IN ($1, $2)
		ORDER BY id
		FOR UPDATE`

	rows, err := tx.Query(ctx, lockQuery, firstAccountID, secondAccountID)
	if err != nil {
		return fmt.Errorf("lock transfer accounts: %w", err)
	}

	balances := make(map[int64]int64, 2)
	for rows.Next() {
		var accountID int64
		var balance int64
		if err := rows.Scan(&accountID, &balance); err != nil {
			rows.Close()
			return fmt.Errorf("scan locked account: %w", err)
		}
		balances[accountID] = balance
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read locked accounts: %w", err)
	}

	if len(balances) != 2 {
		return domain.ErrAccountNotFound
	}

	senderBalance, ok := balances[fromAccountID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	if _, ok := balances[toAccountID]; !ok {
		return domain.ErrAccountNotFound
	}
	if senderBalance < amount {
		return domain.ErrInsufficientFunds
	}

	const debitQuery = `
		UPDATE accounts
		SET balance = balance - $1
		WHERE id = $2 AND balance >= $1`

	commandTag, err := tx.Exec(ctx, debitQuery, amount, fromAccountID)
	if err != nil {
		return fmt.Errorf("debit sender account: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return domain.ErrInsufficientFunds
	}

	const creditQuery = `
		UPDATE accounts
		SET balance = balance + $1
		WHERE id = $2`

	commandTag, err = tx.Exec(ctx, creditQuery, amount, toAccountID)
	if err != nil {
		return fmt.Errorf("credit receiver account: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return domain.ErrAccountNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transfer transaction: %w", err)
	}
	committed = true

	return nil
}
