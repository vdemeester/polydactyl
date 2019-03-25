package main

type Config struct {
	Max         int
	MaxStep     int
	TaskRun     bool
	PipelineRun bool
}

type ConfigOp func(*Config)

func WithMax(max int) ConfigOp {
	return func(c *Config) {
		c.Max = max
	}
}

func WithMaxStep(max int) ConfigOp {
	return func(c *Config) {
		c.MaxStep = max
	}
}

func WithTaskRun(tr bool) ConfigOp {
	return func(c *Config) {
		c.TaskRun = tr
	}
}

func WithPipelineRun(pr bool) ConfigOp {
	return func(c *Config) {
		c.PipelineRun = pr
	}
}
