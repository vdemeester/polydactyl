package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"knative.dev/pkg/injection"
)

const (
	defaultMax     = 10
	defaultMaxStep = 10
)

func main() {
	max := flag.Int("max", defaultMax, "maximum number of run in parallel")
	maxStep := flag.Int("max-step", defaultMaxStep, "maximum number of step in a task")
	taskrun := flag.Bool("taskrun", false, "wether to create taskrun or not")
	pipelinerun := flag.Bool("pipelinerun", false, "wether to create pipelinerun or not")
	namespace := flag.String("namespace", "", "namespace to use (default is empty to create a random one)")

	flag.Parse()

	rand.Seed(time.Now().UTC().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())

	cfg := injection.ParseAndGetRESTConfigOrDie()
	if cfg.QPS == 0 {
		cfg.QPS = 100
	}
	if cfg.Burst == 0 {
		cfg.Burst = 50
	}
	// FIXME(vdemeester): this is here to not break current behavior
	// multiply by 2, no of controllers being created
	cfg.QPS = 2 * cfg.QPS
	cfg.Burst = 2 * cfg.Burst

	ctx, _ = injection.EnableInjectionOrDie(ctx, cfg)

	p, err := Runner(ctx, *namespace, WithMax(*max), WithMaxStep(*maxStep), WithTaskRun(*taskrun), WithPipelineRun(*pipelinerun))
	if err != nil {
		cancel()
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, handle it
			log.Print(sig)
			p.Report(ctx)
			cancel()
		}
	}()

	if err := p.Run(ctx); err != nil {
		cancel()
		fmt.Println("Error:", err)
		os.Exit(1)
	}

}
