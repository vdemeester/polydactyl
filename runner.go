package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	typedv1alpha1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Polydactly struct {
	sync.RWMutex
	config    *Config
	running   int
	namespace string

	kubeClient             *kubernetes.Clientset
	pipelineClient         typedv1alpha1.PipelineInterface
	taskClient             typedv1alpha1.TaskInterface
	taskRunClient          typedv1alpha1.TaskRunInterface
	pipelineRunClient      typedv1alpha1.PipelineRunInterface
	pipelineResourceClient typedv1alpha1.PipelineResourceInterface
}

func Runner(namespace string, opts ...ConfigOp) (*Polydactly, error) {
	cfg := &Config{
		Max:         defaultMax,
		MaxStep:     defaultMaxStep,
		PipelineRun: true,
		TaskRun:     true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	k, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	cs, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	if _, err := k.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		return nil, err
	}

	return &Polydactly{
		config:    cfg,
		namespace: namespace,
		running:   0,

		kubeClient:             k,
		pipelineClient:         cs.TektonV1alpha1().Pipelines(namespace),
		taskClient:             cs.TektonV1alpha1().Tasks(namespace),
		taskRunClient:          cs.TektonV1alpha1().TaskRuns(namespace),
		pipelineRunClient:      cs.TektonV1alpha1().PipelineRuns(namespace),
		pipelineResourceClient: cs.TektonV1alpha1().PipelineResources(namespace),
	}, nil
}

func (p *Polydactly) Run(ctx context.Context) error {
	if !p.config.PipelineRun && !p.config.TaskRun {
		return fmt.Errorf("At least -taskrun or -pipelinerun should be specified")
	}
	max := p.config.Max
	if p.config.PipelineRun && p.config.TaskRun {
		max = p.config.Max / 2
	}
	for {
		var err error
		p.RLock()
		create := max - p.running
		p.RUnlock()
		for i := 0; i < create; i++ {
			numberOfStep := randInt(0, p.config.MaxStep)
			if p.config.PipelineRun {
				if err := p.createPipelineRun(ctx, numberOfStep); err != nil {
					return err
				}
			}
			if p.config.TaskRun {
				if err := p.createTaskRun(ctx, numberOfStep); err != nil {
					return err
				}
			}
		}

		select {
		case <-ctx.Done():
			// Wait for the current run to get done
			return err
		default:
			// let's continue
		}
	}
}

func (p *Polydactly) createPipelineRun(ctx context.Context, steps int) error {
	return nil
}

func (p *Polydactly) createTaskRun(ctx context.Context, steps int) error {
	name := fmt.Sprintf("%s%d", randomString(10), steps)
	fmt.Println("taskrun:", name)
	ops := []tb.TaskSpecOp{}
	for i := 0; i < steps; i++ {
		stepName := fmt.Sprintf("%s%d", "amazing-busybox", i)
		ops = append(ops, tb.Step(stepName, "busybox", tb.Command("/bin/sh"), tb.Args("-c", "sleep 30")))
	}
	taskrun := tb.TaskRun(name, p.namespace, tb.TaskRunSpec(tb.TaskRunTaskSpec(ops...),
		tb.TaskRunTimeout(30*time.Second)))
	if _, err := p.taskRunClient.Create(taskrun); err != nil {
		return fmt.Errorf("Failed to create TaskRun `%s`: %s", "run-giraffe", err)
	}
	p.Lock()
	p.running++
	p.Unlock()
	err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		r, err := p.taskRunClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return func(tr *v1alpha1.TaskRun) (bool, error) {
			cond := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if cond != nil {
				if cond.Status == corev1.ConditionFalse {
					if cond.Reason == "TaskRunTimeout" {
						return true, nil
					}
					return true, fmt.Errorf("taskRun %s completed with the wrong reason: %s", "run-giraffe", cond.Reason)
				} else if cond.Status == corev1.ConditionTrue {
					return true, fmt.Errorf("taskRun %s completed successfully, should have been timed out", "run-giraffe")
				}
			}

			return false, nil

		}(r)
	})
	p.Lock()
	p.running--
	p.Unlock()
	if err != nil {
		return err
	}
	return nil
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return strings.ToLower(string(bytes))
}
