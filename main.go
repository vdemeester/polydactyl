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

	"github.com/tektoncd/pipeline/pkg/names"
)

const (
	defaultMax     = 10
	defaultMaxStep = 10
)

func main() {
	max := flag.Int("max", defaultMax, "maximum number of run in parallel")
	maxStep := flag.Int("max-step", defaultMaxStep, "maximum number of step in a task")
	taskrun := flag.Bool("taskrun", true, "wether to create taskrun or not")
	pipelinerun := flag.Bool("pipelinerun", true, "wether to create pipelinerun or not")

	flag.Parse()

	rand.Seed(time.Now().UTC().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, handle it
			log.Print(sig)
			cancel()
		}
	}()

	namespace := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("polydactyl")
	fmt.Println("working in namespace:", namespace)
	p, err := Runner(namespace, WithMax(*max), WithMaxStep(*maxStep), WithTaskRun(*taskrun), WithPipelineRun(*pipelinerun))
	if err != nil {
		cancel()
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	if err := p.Run(ctx); err != nil {
		cancel()
		fmt.Println("Error:", err)
		os.Exit(1)
	}

}
