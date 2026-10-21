package model

import "testing"

func TestValidate(t *testing.T) {
	tk := Task{Title: "  x  ", Priority: 3}
	if err := tk.Validate(); err != nil || tk.Title != "x" {
		t.Fatal(err, tk.Title)
	}
	if (&Task{Title: "x", Priority: 9}).Validate() != ErrBadPriority {
		t.Fatal("priority")
	}
}
