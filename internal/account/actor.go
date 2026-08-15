package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/afbackend/order-execution-engine/internal/order"
)

var ErrActorStopped = errors.New("actor stopped")

type Actor struct {
	acc  *Account
	in   chan *order.Order
	out  chan *RiskResult
	done chan struct{}
}

func NewActor(accID int64, buyingPower int64) (*Actor, error) {
	acc, err := NewAccount(accID)

	if err != nil {
		return nil, fmt.Errorf("new actor - account create error: %w", err)
	}

	err = acc.SetBuyingPower(buyingPower)
	if err != nil {
		return nil, fmt.Errorf("new actor - account create error: %w", err)
	}

	done := make(chan struct{})
	in := make(chan *order.Order)
	out := make(chan *RiskResult)

	return &Actor{acc: acc, in: in, out: out, done: done}, nil
}

func (a *Actor) processOrder(ord *order.Order) {
	// Unguarded send: the gateway always drains until a.out closes. Assumes the
	// processor's Write never blocks forever; main's shutdown deadline bounds it.

	s, err := a.acc.Reserve(ord)
	r := &RiskResult{OrderID: ord.ID}

	if err != nil {
		r.Status = Rejected
		r.Reason = UnexpectedError
		a.out <- r
		return
	}

	if !s {
		r.Status = Rejected
		r.Reason = InsufficientBuyingPower
		a.out <- r
		return
	}

	r.Status = Accepted
	a.out <- r
}

func (a *Actor) Submit(ctx context.Context, ord *order.Order) error {
	select {
	case a.in <- ord:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-a.done:
		return ErrActorStopped
	}
}

func (a *Actor) Run(ctx context.Context) {
	defer close(a.out)
	defer close(a.done)

	for {
		select {
		case ord := <-a.in:
			a.processOrder(ord)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Actor) Results() <-chan *RiskResult {
	return a.out
}
