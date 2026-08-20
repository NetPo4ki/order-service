package order

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("valid order starts pending", func(t *testing.T) {
		o, err := New(1, 42, 1)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if o.Status() != StatusPending {
			t.Fatalf("Status() = %v, want %v", o.Status(), StatusPending)
		}
		if o.UserID() != 1 || o.ProductID() != 42 || o.Quantity() != 1 {
			t.Fatalf("unexpected fields: %+v", o)
		}
	})

	t.Run("rejects non-positive quantity", func(t *testing.T) {
		if _, err := New(1, 42, 0); !errors.Is(err, ErrInvalidQuantity) {
			t.Fatalf("New(qty=0) error = %v, want %v", err, ErrInvalidQuantity)
		}
		if _, err := New(1, 42, -1); !errors.Is(err, ErrInvalidQuantity) {
			t.Fatalf("New(qty=-1) error = %v, want %v", err, ErrInvalidQuantity)
		}
	})
}

func TestOrder_StateMachine(t *testing.T) {
	t.Run("pending -> confirmed", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		if err := o.Confirm(); err != nil {
			t.Fatalf("Confirm() error = %v, want nil", err)
		}
		if o.Status() != StatusConfirmed {
			t.Fatalf("Status() = %v, want %v", o.Status(), StatusConfirmed)
		}
	})

	t.Run("pending -> cancelled", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		if err := o.Cancel(); err != nil {
			t.Fatalf("Cancel() error = %v, want nil", err)
		}
		if o.Status() != StatusCancelled {
			t.Fatalf("Status() = %v, want %v", o.Status(), StatusCancelled)
		}
	})

	t.Run("confirmed -> cancelled", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		_ = o.Confirm()
		if err := o.Cancel(); err != nil {
			t.Fatalf("Cancel() error = %v, want nil", err)
		}
		if o.Status() != StatusCancelled {
			t.Fatalf("Status() = %v, want %v", o.Status(), StatusCancelled)
		}
	})

	t.Run("cannot confirm twice", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		_ = o.Confirm()
		if err := o.Confirm(); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("second Confirm() error = %v, want %v", err, ErrInvalidTransition)
		}
	})

	t.Run("cannot confirm a cancelled order", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		_ = o.Cancel()
		if err := o.Confirm(); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("Confirm() after Cancel() error = %v, want %v", err, ErrInvalidTransition)
		}
	})

	t.Run("cannot cancel twice", func(t *testing.T) {
		o, _ := New(1, 42, 1)
		_ = o.Cancel()
		if err := o.Cancel(); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("second Cancel() error = %v, want %v", err, ErrInvalidTransition)
		}
	})
}
