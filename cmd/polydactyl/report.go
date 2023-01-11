package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/vdemeester/polydactyl/pkg/report"
	"knative.dev/pkg/injection"
)

type reportOption struct {
	namespace   string
	taskrun     bool
	pipelinerun bool
}

func reportCommand() *cobra.Command {
	opts := reportOption{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "report tekton workloads from the cluster",
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
			return report.Report(ctx, opts.namespace, opts.pipelinerun, opts.taskrun)
		},
	}

	cmd.Flags().StringVar(&opts.namespace, "namespace", "", "Namespace to use (default is empty, to create a random namespace)")
	cmd.Flags().BoolVar(&opts.pipelinerun, "pipelinerun", true, "Wether to create pipelineruns or not")
	cmd.Flags().BoolVar(&opts.taskrun, "taskrun", true, "Wether to create taskruns or not")

	return cmd
}
