package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/afbackend/order-execution-engine/internal/account"
	"github.com/afbackend/order-execution-engine/internal/gateway"
	"github.com/afbackend/order-execution-engine/internal/generator"
	"github.com/afbackend/order-execution-engine/internal/processor"
)

func main() {

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	defer stop()

	actor, err := account.NewActor(1, 10000)
	if err != nil {
		log.Fatalf("actor error: %v", err)
	}

	proc := processor.NewDefault(os.Stdout)
	gate := gateway.NewDefaultGateway(actor.Results(), &proc)
	gen := generator.NewGenerator(actor, 500*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() { defer wg.Done(); gate.Run() }()
	go func() { defer wg.Done(); actor.Run(ctx) }()
	go func() { defer wg.Done(); gen.Run(ctx) }()

	<-ctx.Done()

	log.Println("shutting down")

	wg.Wait()
}
