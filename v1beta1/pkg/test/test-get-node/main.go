package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var config *rest.Config
	var err error
	config, err = rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, _ := kubernetes.NewForConfig(config)
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d nodes in the cluster\n", len(nodes.Items))
	totalGpus := 0
	for _, node := range nodes.Items {
		gpus := node.Status.Capacity["nvidia.com/gpu"]
		fmt.Printf("There are %d gpus in the node: %s\n", gpus.Value(), node.GetName())
		totalGpus += int(gpus.Value())
	}
	fmt.Println(totalGpus)
}
