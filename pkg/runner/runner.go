package runner

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	// "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	typedv1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/clientcmd"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

type Polydactyl struct {
	sync.RWMutex
	config    *Config
	running   int
	namespace string

	kubeClient        kubernetes.Interface
	pipelineClient    typedv1beta1.PipelineInterface
	taskClient        typedv1beta1.TaskInterface
	taskRunClient     typedv1beta1.TaskRunInterface
	pipelineRunClient typedv1beta1.PipelineRunInterface
}

func New(ctx context.Context, namespace string, opts ...ConfigOp) (*Polydactyl, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	cfg := &Config{
		PipelineRun: true, // default to true
		TaskRun:     true, // default to true
	}

	for _, opt := range opts {
		opt(cfg)
	}

	k := kubeclient.Get(ctx)
	cs := pipelineclient.Get(ctx)

	if namespace == "" {
		namespace := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("polydactyl")
		if _, err := k.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	fmt.Println("working in namespace:", namespace)

	return &Polydactyl{
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

func (p *Polydactyl) Run(ctx context.Context) error {
	// Give time for any "namespace" mutation to take place
	time.Sleep(30 * time.Second)
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
				c := make(chan struct{})
				go func() {
					c <- struct{}{}
					if name, err := p.createPipelineRun(ctx, numberOfStep); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating pipelinerun %q: %v\n", name, err)
					}
				}()
				<-c
			}
			if p.config.TaskRun {
				c := make(chan struct{})
				go func() {
					c <- struct{}{}
					if name, err := p.createTaskRun(ctx, numberOfStep); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating taskrun %q: %v\n", name, err)
					}
				}()
				<-c
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

func (p *Polydactyl) Report(ctx context.Context) {
	if p.config.PipelineRun {
		prs, err := p.pipelineRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing pipelineruns: %v\n", err)
		}
		fmt.Fprintf(os.Stdout, "name: start -> completion\n")
		for _, pr := range prs.Items {
			fmt.Fprintf(os.Stdout, "%s: %v -> %v\n",
				pr.Name, pr.Status.StartTime, pr.Status.CompletionTime,
			)
		}
	}
	if p.config.TaskRun {
		trs, err := p.taskRunClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing taskruns: %v", err)
		}
		pods, err := p.kubeClient.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing pods: %v", err)
		}
		podByName := map[string]corev1.Pod{}
		for _, pod := range pods.Items {
			podByName[pod.Name] = pod
		}
		fmt.Fprintf(os.Stdout, "name: start -> completion\n")
		for _, tr := range trs.Items {
			pod := podByName[tr.Status.PodName]
			fmt.Fprintf(os.Stdout, "%s: %v (%v) -> %v\n",
				tr.Name, tr.Status.StartTime,
				pod.Status.StartTime,
				tr.Status.CompletionTime,
			)
		}
	}
}

func (p *Polydactyl) createPipelineRun(ctx context.Context, steps int) (string, error) {
	// name := fmt.Sprintf("%s%d", randomString(10), steps)
	// The Pipeline should do something decent, not too long, not too short
	// Let's make it 2 task
	// - git-clone
	// - golang-build
	return "", nil
}

func (p *Polydactyl) createTaskRun(ctx context.Context, steps int) (string, error) {
	name := fmt.Sprintf("%s%d", randomString(10), steps)
	fmt.Println("taskrun:", name)
	tasksteps := []v1beta1.Step{}
	for i := 0; i < steps; i++ {
		tasksteps = append(tasksteps, v1beta1.Step{
			Name:    fmt.Sprintf("%s%d", "amazing-ubi", i),
			Image:   "registry.access.redhat.com/ubi8/ubi@sha256:0234b7c6e5696435e8759e914ed19e80e595a89f380e1d0b5d324d71b7041a13",
			Script: "#!/usr/bin/envv bash\nsleep 60",
			//Command: []string{"/bin/sh"},
			//Args:    []string{"-c", "sleep 60"},
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
	if _, err := p.taskRunClient.Create(ctx, taskrun, metav1.CreateOptions{}); err != nil {
		return name, fmt.Errorf("Failed to create TaskRun `%s`: %s", "run-giraffe", err)
	}
	p.Lock()
	p.running++
	p.Unlock()
	err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		r, err := p.taskRunClient.Get(ctx, name, metav1.GetOptions{})
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
