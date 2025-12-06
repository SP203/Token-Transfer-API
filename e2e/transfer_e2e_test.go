package e2e

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	appdb "github.com/SP203/Token-Transfer-API/internal/db"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BTP_DB_DSN")
	if dsn == "" {
		dsn = "postgres://btp:btp@localhost:5432/btp"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := appdb.Init(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func resetWallet(t *testing.T, db *sql.DB, addr string, bal int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO wallets(address, balance) VALUES($1,$2)
	                    ON CONFLICT (address) DO UPDATE SET balance=$2`, addr, bal)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransferOK(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	resetWallet(t, db, "0xA", 100)
	resetWallet(t, db, "0xB", 0)

	repo := appdb.NewRepo(db)
	newBal, err := repo.Transfer(context.Background(), "0xA", "0xB", 30)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}
	if newBal != 70 {
		t.Fatalf("want 70, got %d", newBal)
	}
}

func TestTransferInsufficient(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	resetWallet(t, db, "0xA", 10)
	resetWallet(t, db, "0xB", 0)

	repo := appdb.NewRepo(db)
	_, err := repo.Transfer(context.Background(), "0xA", "0xB", 50)
	if err == nil || err.Error() != appdb.ErrInsufficient.Error() {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
}

func TestTransferRaceScenario(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	resetWallet(t, db, "0xX", 10)
	resetWallet(t, db, "0xD", 5)
	resetWallet(t, db, "0xY", 0)
	resetWallet(t, db, "0xZ", 0)

	repo := appdb.NewRepo(db)

	var ready int32
	allReady := make(chan struct{})
	release := make(chan struct{})

	repo.PreWriteHook = func() {
		if atomic.AddInt32(&ready, 1) == 3 {
			close(allReady)
		}
		<-release
	}

	errs := make(chan error, 3)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		_, err := repo.Transfer(context.Background(), "0xX", "0xY", 4)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := repo.Transfer(context.Background(), "0xX", "0xZ", 7)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := repo.Transfer(context.Background(), "0xD", "0xX", 1)
		errs <- err
	}()

	select {
	case <-allReady:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not reach precondition barrier")
	}

	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err == nil {
			continue
		}
		if !errors.Is(err, appdb.ErrInsufficient) {
			t.Fatalf("unexpected error returned: %v", err)
		}
	}

	var final int64
	if err := db.QueryRow(`SELECT balance FROM wallets WHERE address=$1`, "0xX").Scan(&final); err != nil {
		t.Fatal(err)
	}

	if final != 7 && final != 4 && final != 0 {
		t.Fatalf("unexpected final balance %d", final)
	}
}
