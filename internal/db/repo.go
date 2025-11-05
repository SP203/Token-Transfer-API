package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}

	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 40001") || strings.Contains(msg, "SQLSTATE 40P01")
}

func (r *Repo) Transfer(ctx context.Context, from, to string, amount int64) (int64, error) {

	for attempt := 0; attempt < 3; attempt++ {
		newBal, err := r.transferOnce(ctx, from, to, amount)
		if err == nil {
			return newBal, nil
		}
		if isRetryable(err) {

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
		var bal int64
		_ = r.DB.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1`, from).Scan(&bal)
		return bal, nil
	}

	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var preBal int64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1`, from).Scan(&preBal)
	if err == sql.ErrNoRows {
		preBal = 0
	} else if err != nil {
		return 0, err
	}

	if preBal < amount {
		return 0, ErrInsufficient
	}

	if r.PreWriteHook != nil {
		r.PreWriteHook()
	}

	var fromBal int64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1 FOR UPDATE`, from).Scan(&fromBal)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, `INSERT INTO wallets(address, balance) VALUES($1, 0)`, from); err != nil {
			return 0, err
		}
		fromBal = 0
	} else if err != nil {
		return 0, err
	}

	if fromBal < amount {
		return 0, ErrInsufficient
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO wallets(address, balance) VALUES($1, 0)
		ON CONFLICT (address) DO NOTHING
	`, to); err != nil {
		return 0, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE wallets SET balance = balance - $1 WHERE address=$2`,
		amount, from,
	); err != nil {
		return 0, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE address=$2`,
		amount, to,
	); err != nil {
		return 0, err
	}

	var newFrom int64
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1`, from).Scan(&newFrom); err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return newFrom, nil
}
