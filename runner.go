package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	typedv1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/apis"
)

type Polydactly struct {
	sync.RWMutex
	config    *Config
	running   int
	namespace string

	kubeClient        *kubernetes.Clientset
	pipelineClient    typedv1beta1.PipelineInterface
	taskClient        typedv1beta1.TaskInterface
	taskRunClient     typedv1beta1.TaskRunInterface
	pipelineRunClient typedv1beta1.PipelineRunInterface
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

	if _, err := k.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		return nil, err
	}

	return &Polydactly{
		config:    cfg,
		namespace: namespace,
		running:   0,

		kubeClient:        k,
		pipelineClient:    cs.TektonV1beta1().Pipelines(namespace),
		taskClient:        cs.TektonV1beta1().Tasks(namespace),
		taskRunClient:     cs.TektonV1beta1().TaskRuns(namespace),
		pipelineRunClient: cs.TektonV1beta1().PipelineRuns(namespace),
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
			numberOfStep := randInt(1, p.config.MaxStep)
			if p.config.PipelineRun {
				go func() {
					if name, err := p.createPipelineRun(ctx, numberOfStep); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating pipelinerun %q: %v\n", name, err)
					}
				}()
			}
			if p.config.TaskRun {
				go func() {
					if name, err := p.createTaskRun(ctx, numberOfStep); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating taskrun %q: %v\n", name, err)
					}
				}()
			}
		}

		select {
		case <-ctx.Done():
			// Wait for the current run to get done
			return err
		case <-time.After(10 * time.Second):
			// let's continue
		}
	}
}

func (p *Polydactly) createPipelineRun(ctx context.Context, steps int) (string, error) {
	return "", nil
}

func (p *Polydactly) createTaskRun(ctx context.Context, steps int) (string, error) {
	name := fmt.Sprintf("%s%d", randomString(10), steps)
	fmt.Println("taskrun:", name)
	tasksteps := []v1beta1.Step{}
	for i := 0; i < steps; i++ {
		tasksteps = append(tasksteps, v1beta1.Step{
			Name:    fmt.Sprintf("%s%d", "amazing-busybox", i),
			Command: []string{"/bin/sh"},
			Args:    []string{"-c", "sleep 60"},
		})
	}
	taskrun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: tasksteps,
			},
		},
	}
	if _, err := p.taskRunClient.Create(context.Background(), taskrun, metav1.CreateOptions{}); err != nil {
		return name, fmt.Errorf("Failed to create TaskRun `%s`: %s", "run-giraffe", err)
	}
	p.Lock()
	p.running++
	p.Unlock()
	err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		r, err := p.taskRunClient.Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return func(tr *v1beta1.TaskRun) (bool, error) {
			cond := tr.Status.GetCondition(apis.ConditionSucceeded)
			if cond != nil {
				if cond.Status == corev1.ConditionFalse {
					if cond.Reason == "TaskRunTimeout" {
						return true, nil
					}
					return true, fmt.Errorf("taskRun %s completed with the wrong reason: %s", name, cond.Reason)
				} else if cond.Status == corev1.ConditionTrue {
					return true, fmt.Errorf("taskRun %s completed successfully, should have been timed out", name)
				}
			}

			return false, nil

		}(r)
	})
	p.Lock()
	p.running--
	p.Unlock()
	if err != nil {
		return name, err
	}
	return name, nil
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
