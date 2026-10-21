package bank

import (
	"errors"
	"sync"
	"testing"
)

func TestTransfer(t *testing.T) {
	b := New(100, 0)
	if err := b.Transfer(0, 1, 30); err != nil {
		t.Fatal(err)
	}
	a0, _ := b.Account(0)
	a1, _ := b.Account(1)
	if a0.Balance() != 70 || a1.Balance() != 30 {
		t.Fatal("balances")
	}
	if !errors.Is(b.Transfer(1, 0, 31), ErrInsufficient) {
		t.Fatal("expected insufficient")
	}
}

func TestOpposingTransfers(t *testing.T) {
	b := New(1000, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); for j := 0; j < 200; j++ { b.Transfer(0, 1, 1) } }()
		go func() { defer wg.Done(); for j := 0; j < 200; j++ { b.Transfer(1, 0, 1) } }()
	}
	wg.Wait()
	if b.Total() != 2000 {
		t.Fatal("money lost")
	}
}
