package wallet

import (
	"errors"
	"os"
	"testing"

	"github.com/AdventurerAmer/payments/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// newStore connects to the database in WALLET_TEST_DATABASE_URL. Without it the
// test is skipped, so go test ./... still passes on a machine without Docker.
func newStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("WALLET_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("WALLET_TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := postgres.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewStore(pool)
}

// newAccount opens a fresh USD account, so every test works on its own data.
func newAccount(t *testing.T, s *Store, name string, side Direction) uuid.UUID {
	t.Helper()
	acc, err := s.CreateAccount(t.Context(), NewAccount{
		Name:          name,
		Currency:      "USD",
		NormalBalance: side,
	})
	if err != nil {
		t.Fatalf("create account %s: %v", name, err)
	}
	return acc.ID
}

// balance returns an account's current balance in cents.
func balance(t *testing.T, s *Store, id uuid.UUID) int64 {
	t.Helper()
	b, err := s.Balance(t.Context(), id)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b.Balance
}

func TestPostTransaction(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	bank := newAccount(t, s, "bank", Debit)
	alice := newAccount(t, s, "alice", Credit)

	tx, err := s.PostTransaction(t.Context(), NewTransaction{
		Description: "alice deposits $50",
		Postings: []Posting{
			{AccountID: bank, Direction: Debit, Amount: 5000, Currency: "USD"},
			{AccountID: alice, Direction: Credit, Amount: 5000, Currency: "USD"},
		},
	})
	if err != nil {
		t.Fatalf("post transaction: %v", err)
	}

	if got := balance(t, s, bank); got != 5000 {
		t.Errorf("bank balance = %d, want 5000", got)
	}
	if got := balance(t, s, alice); got != 5000 {
		t.Errorf("alice balance = %d, want 5000", got)
	}

	entries, err := s.Entries(t.Context(), alice, 0, 10)
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 1 || entries[0].TransactionID != tx.ID {
		t.Errorf("alice has %d entries, want 1 in transaction %s", len(entries), tx.ID)
	}
}

func TestPostTransactionRejectsUnbalanced(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	alice := newAccount(t, s, "alice", Credit)
	bob := newAccount(t, s, "bob", Credit)

	// Alice pays $20, but Bob only receives $15: five dollars would vanish.
	_, err := s.PostTransaction(t.Context(), NewTransaction{
		Description: "alice pays bob $20",
		Postings: []Posting{
			{AccountID: alice, Direction: Debit, Amount: 2000, Currency: "USD"},
			{AccountID: bob, Direction: Credit, Amount: 1500, Currency: "USD"},
		},
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("got error %v, want a check_violation (23514) from Postgres", err)
	}
	t.Logf("rejected by Postgres: %s", pgErr.Message)

	// Atomic: the entries inserted before the failed commit were rolled back too.
	if got := balance(t, s, alice); got != 0 {
		t.Errorf("alice balance = %d, want 0", got)
	}
	if got := balance(t, s, bob); got != 0 {
		t.Errorf("bob balance = %d, want 0", got)
	}
}

func TestAccountNotFound(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	_, err := s.Account(t.Context(), uuid.New())
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("got error %v, want ErrAccountNotFound", err)
	}
}
