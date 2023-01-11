package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/vdemeester/polydactyl/pkg/runner"
	"knative.dev/pkg/injection"
)

const (
	defaultMax     = 10
	defaultMaxStep = 10
)

type generateOption struct {
	max         int
	maxStep     int
	taskrun     bool
	pipelinerun bool
	namespace   string
}

func generateCommand() *cobra.Command {
	opts := generateOption{}
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "generate some \"tekton\" loads on the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			p, err := runner.New(ctx, opts.namespace, runner.WithMax(opts.max), runner.WithMaxStep(opts.maxStep), runner.WithTaskRun(opts.taskrun), runner.WithPipelineRun(opts.pipelinerun))
			if err != nil {
				return err
			}
			return p.Run(ctx)
		},
	}
	cmd.Flags().StringVar(&opts.namespace, "namespace", "", "Namespace to use (default is empty, to create a random namespace)")

	cmd.Flags().IntVar(&opts.max, "max", defaultMax, "Maximum number of runs in parallel")
	cmd.Flags().IntVar(&opts.maxStep, "max-step", defaultMaxStep, "Maximum number of step in a task")

	cmd.Flags().BoolVar(&opts.pipelinerun, "pipelinerun", true, "Wether to create pipelineruns or not")
	cmd.Flags().BoolVar(&opts.taskrun, "taskrun", true, "Wether to create taskruns or not")

	return cmd
}
