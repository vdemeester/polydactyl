package report

import (
	"context"
	"fmt"
	"os"

	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

func Report(ctx context.Context, namespace string, pipelinerun bool, taskrun bool) error {
	k := kubeclient.Get(ctx)
	cs := pipelineclient.Get(ctx)

	if pipelinerun {
		prs, err := cs.TektonV1beta1().PipelineRuns(namespace).List(ctx, metav1.ListOptions{})
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
	if taskrun {
		trs, err := cs.TektonV1beta1().TaskRuns(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing taskruns: %v", err)
		}
		pods, err := k.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
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
	return nil
}
