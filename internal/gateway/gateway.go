package gateway

import (
	"log"

	"github.com/afbackend/order-execution-engine/internal/account"
)

type RiskProcessor interface {
	RiskResult(result *account.RiskResult) error
}

type Gateway struct {
	processor RiskProcessor
	in        <-chan *account.RiskResult
}

func NewDefaultGateway(in <-chan *account.RiskResult, processor RiskProcessor) *Gateway {
	return &Gateway{in: in, processor: processor}
}

func (g *Gateway) process(result *account.RiskResult) {
	err := g.processor.RiskResult(result)
	if err != nil {
		log.Printf("Error in Gateway.process: %v", err)
	}
}

// Run drains until in closes. It must not select on ctx.Done(): exiting early
// would strand the actor mid-send and hang shutdown.
func (g *Gateway) Run() {
	for {
		select {
		case r, ok := <-g.in:
			if !ok {
				return
			}

			g.process(r)
		}
	}
}
