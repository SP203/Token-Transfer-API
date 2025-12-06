package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrInsufficient = errors.New("insufficient balance")

type Repo struct {
	DB           *sql.DB
	PreWriteHook func()
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{DB: db}
}

func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001"
	}
	return strings.Contains(err.Error(), "SQLSTATE 40001")
}

func (r *Repo) Transfer(ctx context.Context, from, to string, amount int64) (int64, error) {
	for attempt := 0; attempt < 3; attempt++ {
		newBal, err := r.transferOnce(ctx, from, to, amount)
		if err == nil {
			return newBal, nil
		}

		if isRetryable(err) {
			delay := time.Duration(5+rand.Intn(20)) * time.Millisecond
			time.Sleep(delay)
			continue
		}

		return 0, err
	}

	return 0, fmt.Errorf("could not serialize transfer after retries")
}

func (r *Repo) transferOnce(ctx context.Context, from, to string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}

	if from == to {
		return r.getBalance(ctx, from)
	}

	addr1, addr2 := orderAddresses(from, to)

	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	bal1, err := r.loadOrCreateWallet(ctx, tx, addr1, to)
	if err != nil {
		return 0, err
	}

	bal2, err := r.loadOrCreateWallet(ctx, tx, addr2, to)
	if err != nil {
		return 0, err
	}

	fromBal, toBal := r.mapBalances(from, addr1, &bal1, &bal2)

	if *fromBal < amount {
		return 0, ErrInsufficient
	}

	if r.PreWriteHook != nil {
		r.PreWriteHook()
	}

	*fromBal -= amount
	*toBal += amount

	if err := r.updateBalances(ctx, tx, addr1, bal1, addr2, bal2); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if from == addr1 {
		return bal1, nil
	}
	return bal2, nil
}

func (r *Repo) updateBalances(ctx context.Context, tx *sql.Tx,
	addr1 string, bal1 int64,
	addr2 string, bal2 int64) error {

	if _, err := tx.ExecContext(ctx,
		`UPDATE wallets SET balance=$1 WHERE address=$2`,
		bal1, addr1,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE wallets SET balance=$1 WHERE address=$2`,
		bal2, addr2,
	); err != nil {
		return err
	}

	return nil
}

func (r *Repo) mapBalances(from, addr1 string, bal1, bal2 *int64) (*int64, *int64) {
	if from == addr1 {
		return bal1, bal2
	}
	return bal2, bal1
}

func (r *Repo) loadOrCreateWallet(ctx context.Context, tx *sql.Tx, address, to string) (int64, error) {
	var bal int64

	err := tx.QueryRowContext(ctx,
		`SELECT balance FROM wallets WHERE address=$1 FOR UPDATE`,
		address,
	).Scan(&bal)

	if err == sql.ErrNoRows {
		if address == to {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO wallets(address, balance) VALUES($1, 0)`,
				address,
			)
			if err != nil {
				return 0, err
			}
			return 0, nil
		}
		return 0, ErrInsufficient
	}

	if err != nil {
		return 0, err
	}

	return bal, nil
}

func (r *Repo) getBalance(ctx context.Context, address string) (int64, error) {
	var bal int64
	err := r.DB.QueryRowContext(ctx,
		`SELECT balance FROM wallets WHERE address=$1`, address,
	).Scan(&bal)
	if err != nil {
		return 0, err
	}
	return bal, nil
}

func orderAddresses(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}
