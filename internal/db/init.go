package db

import (
	"context"
	"database/sql"
)

const GenesisAddress = "0x0000000000000000000000000000000000000000"
const GenesisBalance int64 = 1_000_000

const createTable = `
CREATE TABLE IF NOT EXISTS wallets (
  address TEXT PRIMARY KEY,
  balance BIGINT NOT NULL CHECK (balance >= 0)
);`

const upsertGenesis = `
INSERT INTO wallets(address, balance)
VALUES ($1, $2)
ON CONFLICT (address) DO NOTHING;`

func Init(ctx context.Context, d *sql.DB) error {
	if _, err := d.ExecContext(ctx, createTable); err != nil {
		return err
	}
	_, err := d.ExecContext(ctx, upsertGenesis, GenesisAddress, GenesisBalance)
	return err
}
