package generator

import (
	"context"
	"log"
	"time"

	"github.com/afbackend/pretrade-risk-engine/internal/account"
	"github.com/afbackend/pretrade-risk-engine/internal/order"
)

type Generator struct {
	actor    *account.Actor
	orders   []*order.Order
	interval time.Duration
}

func NewGenerator(actor *account.Actor, interval time.Duration) *Generator {
	return &Generator{
		actor:    actor,
		orders:   buildOrders(),
		interval: interval,
	}
}

func (g *Generator) Run(ctx context.Context) {
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ord := g.orders[i%len(g.orders)]
			if err := g.actor.Submit(ctx, ord); err != nil {
				return
			}
			i++
		}
	}
}

func buildOrders() []*order.Order {
	mk := func(id, price, qty int64) *order.Order {
		ord, err := order.NewOrder(id, price, qty, "BTC", order.Long)
		if err != nil {
			log.Fatalf("invalid order in fixture: %v", err)
		}
		return ord
	}

	return []*order.Order{
		mk(1, 1000, 1),
		mk(2, 5000, 2),
		mk(3, 900000, 100),
		mk(4, 2000, 1),
	}
}
