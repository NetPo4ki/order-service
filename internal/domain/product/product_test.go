package product

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	cases := []struct {
		name       string
		productArg string
		price      int64
		stock      int
		wantErr    error
	}{
		{"valid", "Chair", 1999, 5, nil},
		{"empty name", "", 1999, 5, ErrInvalidName},
		{"zero price", "Chair", 0, 5, ErrInvalidPrice},
		{"negative price", "Chair", -100, 5, ErrInvalidPrice},
		{"negative stock", "Chair", 1999, -1, ErrInvalidStock},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.productArg, tc.price, tc.stock)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("New() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && p == nil {
				t.Fatal("expected non-nil product on success")
			}
		})
	}
}

func TestProduct_Reserve(t *testing.T) {
	t.Run("reserves within stock", func(t *testing.T) {
		p, err := New("Chair", 1999, 1)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := p.Reserve(1); err != nil {
			t.Fatalf("Reserve() error = %v, want nil", err)
		}
		if p.Stock() != 0 {
			t.Fatalf("Stock() = %d, want 0", p.Stock())
		}
	})

	t.Run("rejects when out of stock", func(t *testing.T) {
		p, err := New("Chair", 1999, 1)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := p.Reserve(2); !errors.Is(err, ErrOutOfStock) {
			t.Fatalf("Reserve() error = %v, want %v", err, ErrOutOfStock)
		}
		// стейт не должен измениться при отклонённой резервации
		if p.Stock() != 1 {
			t.Fatalf("Stock() = %d, want unchanged 1", p.Stock())
		}
	})

	t.Run("rejects non-positive quantity", func(t *testing.T) {
		p, err := New("Chair", 1999, 1)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := p.Reserve(0); !errors.Is(err, ErrInvalidQuantity) {
			t.Fatalf("Reserve(0) error = %v, want %v", err, ErrInvalidQuantity)
		}
		if err := p.Reserve(-1); !errors.Is(err, ErrInvalidQuantity) {
			t.Fatalf("Reserve(-1) error = %v, want %v", err, ErrInvalidQuantity)
		}
	})

	t.Run("last-item race is not a concurrency guarantee at this layer", func(t *testing.T) {
		p1 := Rehydrate(1, "Chair", 1999, 1)
		p2 := Rehydrate(1, "Chair", 1999, 1)

		if err := p1.Reserve(1); err != nil {
			t.Fatalf("p1.Reserve() error = %v", err)
		}
		if err := p2.Reserve(1); err != nil {
			t.Fatalf("p2.Reserve() error = %v", err)
		}
	})
}
