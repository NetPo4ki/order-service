package product

import "errors"

var (
	ErrInvalidPrice    = errors.New("product: price must be positive")
	ErrInvalidStock    = errors.New("product: stock cannot be negative")
	ErrInvalidName     = errors.New("product: name is required")
	ErrInvalidQuantity = errors.New("product: quantity must be positive")
	ErrOutOfStock      = errors.New("product: not enough stock")
)

type Product struct {
	id    int64
	name  string
	price int64
	stock int
}

func New(name string, priceCents int64, stock int) (*Product, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if priceCents <= 0 {
		return nil, ErrInvalidPrice
	}
	if stock < 0 {
		return nil, ErrInvalidStock
	}
	return &Product{name: name, price: priceCents, stock: stock}, nil
}

func Rehydrate(id int64, name string, priceCents int64, stock int) *Product {
	return &Product{id: id, name: name, price: priceCents, stock: stock}
}

func (p *Product) ID() int64         { return p.id }
func (p *Product) Name() string      { return p.name }
func (p *Product) PriceCents() int64 { return p.price }
func (p *Product) Stock() int        { return p.stock }

func (p *Product) CanReserve(qty int) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	if p.stock < qty {
		return ErrOutOfStock
	}
	return nil
}

func (p *Product) Reserve(qty int) error {
	if err := p.CanReserve(qty); err != nil {
		return err
	}
	p.stock -= qty
	return nil
}
