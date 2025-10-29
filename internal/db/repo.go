package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrInsufficient = errors.New("insufficient balance")

type Repo struct {
	DB *sql.DB
}

func NewRepo(db *sql.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) GetBalance(ctx context.Context, address string) (int64, error) {
	var bal int64
	err := r.DB.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1`, address).Scan(&bal)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return bal, err
}

// Transfer wykonuje atomowy przelew i zwraca nowe saldo nadawcy.
func (r *Repo) Transfer(ctx context.Context, from, to string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	if from == to {
		return r.GetBalance(ctx, from)
	}

	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Zablokuj (FOR UPDATE) i upewnij się, że wiersz nadawcy istnieje
	var fromBal int64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1 FOR UPDATE`, from).Scan(&fromBal)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, `INSERT INTO wallets(address, balance) VALUES($1, 0)`, from); err != nil {
			return 0, err
		}
		err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1 FOR UPDATE`, from).Scan(&fromBal)
	}
	if err != nil {
		return 0, err
	}

	// Wystarczająca ilość środków?
	if fromBal < amount {
		return 0, ErrInsufficient
	}

	// Upewnij się, że odbiorca istnieje
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO wallets(address, balance) VALUES($1, 0)
		ON CONFLICT (address) DO NOTHING
	`, to); err != nil {
		return 0, err
	}

	// Aktualizacje sald
	if _, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1 WHERE address=$2`, amount, from); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance + $1 WHERE address=$2`, amount, to); err != nil {
		return 0, err
	}

	// Nowe saldo nadawcy
	var newFrom int64
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE address=$1`, from).Scan(&newFrom); err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return newFrom, nil
}
