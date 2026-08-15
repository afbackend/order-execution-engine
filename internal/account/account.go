package account

import (
	"errors"

	"github.com/afbackend/order-execution-engine/internal/order"
)

var ErrInvalidBuyingPower = errors.New("invalid buying power")
var ErrNotionalOverflow = errors.New("notional overflow")

type RiskStatus int8

const (
	Pending RiskStatus = iota
	Accepted
	Rejected
)

type RiskReason int8

const (
	None RiskReason = iota
	InsufficientBuyingPower
	UnexpectedError
)

type RiskResult struct {
	OrderID int64
	Status  RiskStatus
	Reason  RiskReason
}

type Account struct {
	ID          int64
	buyingPower int64
	exposure    int64
}

func NewAccount(id int64) (*Account, error) {
	return &Account{ID: id}, nil
}

func notional(price, qty, scale int64) (int64, error) {
	if price != 0 && (price*qty)/price != qty {
		return 0, ErrNotionalOverflow
	}
	product := price * qty
	result := product / scale
	if product%scale != 0 {
		result++ // ceil
	}
	return result, nil
}

func (ac *Account) SetBuyingPower(value int64) error {
	if value < 0 {
		return ErrInvalidBuyingPower
	}

	ac.buyingPower = value
	return nil
}

func (ac *Account) Reserve(ord *order.Order) (bool, error) {
	n, err := notional(ord.Price, ord.Qty, order.SCALE)

	if err != nil {
		return false, err
	}

	if ac.exposure+n > ac.buyingPower {
		return false, nil
	}

	ac.exposure += n

	return true, nil
}
