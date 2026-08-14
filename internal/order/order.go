package order

import (
	"errors"
	"fmt"
)

const SCALE = 100

var ErrInvalidPrice = errors.New("invalid order price")
var ErrInvalidQty = errors.New("invalid quantity")
var ErrInvalidSymbol = errors.New("invalid symbol")

type Side int8

const (
	Long Side = iota
	Short
)

type Order struct {
	ID     int64
	Price  int64
	Qty    int64
	Symbol string
	Side   Side
}

func NewOrder(id int64, price int64, qty int64, symbol string, side Side) (*Order, error) {

	if price <= 0 {
		return nil, fmt.Errorf("fail to create new order: %w", ErrInvalidPrice)
	}

	if qty <= 0 {
		return nil, fmt.Errorf("fail to create new order: %w", ErrInvalidQty)
	}

	if symbol == "" {
		return nil, fmt.Errorf("fail to create new order: %w", ErrInvalidSymbol)
	}

	return &Order{ID: id, Price: price, Qty: qty, Symbol: symbol, Side: side}, nil
}
