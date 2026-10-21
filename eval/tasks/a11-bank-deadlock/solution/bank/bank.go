// Package bank models accounts with per-account locks.
package bank

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrInsufficient = errors.New("bank: insufficient funds")
	ErrSameAccount  = errors.New("bank: same account")
	ErrBadAmount    = errors.New("bank: amount must be positive")
	ErrNoAccount    = errors.New("bank: no such account")
)

// Account holds a balance in cents.
type Account struct {
	ID      int
	mu      sync.Mutex
	balance int64
}

// Balance returns the current balance.
func (a *Account) Balance() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}

// Bank is a set of accounts.
type Bank struct {
	accounts map[int]*Account
}

// New creates a bank with the given opening balances, IDs 0..n-1.
func New(balances ...int64) *Bank {
	b := &Bank{accounts: map[int]*Account{}}
	for i, bal := range balances {
		b.accounts[i] = &Account{ID: i, balance: bal}
	}
	return b
}

// Account returns an account by ID.
func (b *Bank) Account(id int) (*Account, error) {
	a, ok := b.accounts[id]
	if !ok {
		return nil, ErrNoAccount
	}
	return a, nil
}

// Transfer moves amount from one account to another atomically.
func (b *Bank) Transfer(from, to int, amount int64) error {
	if amount <= 0 {
		return ErrBadAmount
	}
	if from == to {
		return ErrSameAccount
	}
	src, err := b.Account(from)
	if err != nil {
		return err
	}
	dst, err := b.Account(to)
	if err != nil {
		return err
	}
	first, second := src, dst
	if dst.ID < src.ID {
		first, second = dst, src
	}
	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()
	if src.balance < amount {
		return ErrInsufficient
	}
	src.balance -= amount
	dst.balance += amount
	return nil
}

// Total sums all balances.
func (b *Bank) Total() int64 {
	ids := make([]int, 0, len(b.accounts))
	for id := range b.accounts {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		b.accounts[id].mu.Lock()
	}
	var t int64
	for _, id := range ids {
		t += b.accounts[id].balance
	}
	for _, id := range ids {
		b.accounts[id].mu.Unlock()
	}
	return t
}
