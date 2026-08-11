package venue

import (
	"errors"
	"fmt"
)

var ErrInvalidPrice = errors.New("invalid order price")
var ErrInvalidQty = errors.New("invalid quantity")
var ErrInvalidSymbol = errors.New("invalid symbol")

type Side int8

const (
	Long Side = iota
	Short
)

type Order struct {
	id     int64
	price  int64
	qty    int64
	symbol string
	side   Side
}

type Fill struct {
	id      int64
	price   int64
	qty     int64
	orderID int64
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

	return &Order{id, price, qty, symbol, side}, nil
}

func Execute(order *Order) (*Fill, error) {
	return &Fill{1, order.price, order.qty, order.id}, nil
}
