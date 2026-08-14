package gateway

import (
	"context"
	"log"

	"github.com/afbackend/order-execution-engine/internal/account"
	"github.com/afbackend/order-execution-engine/internal/processor"
)

type Gateway struct {
	processor processor.RiskProcessor
	in        <-chan *account.RiskResult
}

func NewGateway(in <-chan *account.RiskResult, processor processor.RiskProcessor) *Gateway {
	return &Gateway{in: in, processor: processor}
}

func (g *Gateway) process(result *account.RiskResult) {
	err := g.processor.RiskResult(result)
	if err != nil {
		log.Printf("Error in Gateway.process: %v", err)
	}
}

func (g *Gateway) Run(ctx context.Context) {
	for {
		select {
		case r, ok := <-g.in:
			if !ok {
				return
			}

			g.process(r)
		case <-ctx.Done():
			return
		}
	}
}
