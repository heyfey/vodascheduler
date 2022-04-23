package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

var toleration = corev1.Toleration{
	Key:      "vodascheduler/hostname",
	Value:    "gpu3",
	Operator: corev1.TolerationOpEqual,
	Effect:   corev1.TaintEffectNoExecute,
}

func main() {
	var config *rest.Config
	var err error
	config, err = rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, _ := kubernetes.NewForConfig(config)

	// test update annotation at runtime
	// err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
	// 	pod, err := clientset.CoreV1().Pods("default").Get("tensorflow2-keras-mnist-elastic-launcher", metav1.GetOptions{})
	// 	// if !HasAnnotation(pod, "voda-scheduler-dummy-key") {
	// 	if pod.GetAnnotations() == nil {
	// 		m := make(map[string]string)
	// 		m["voda-scheduler-dummy-key"] = "dummy-value"
	// 		pod.SetAnnotations(m)
	// 		// SetMetaDataAnnotation(pod, "voda-scheduler-dummy-key", "dummy-value")
	// 	} else {
	// 		pod.Annotations["voda-scheduler-dummy-key"] = "dummy-value"
	// 		// SetMetaDataAnnotation(pod, "voda-scheduler-dummy-key", "dummy-value")
	// 	}
	// 	_, err = clientset.CoreV1().Pods("default").Update(pod)
	// 	return err
	// })

	// test update toleration at runtime
	// err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
	// 	pod, err := clientset.CoreV1().Pods("default").Get(context.TODO(), "tensorflow2-keras-mnist-elastic-worker-1", metav1.GetOptions{})
	// 	// tolerations := []corev1.Toleration{}
	// 	// pod.Spec.Tolerations = tolerations
	// 	for _, t := range pod.Spec.Tolerations {
	// 		if t.MatchToleration(&toleration) {
	// 			fmt.Println("match")
	// 		}
	// 	}
	// 	_, err = clientset.CoreV1().Pods("default").Update(context.TODO(), pod, metav1.UpdateOptions{})
	// 	return err
	// })
	// if err != nil {
	// 	panic(err.Error())
	// }
	// err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
	// 	pod, err := clientset.CoreV1().Pods("default").Get(context.TODO(), "tensorflow2-keras-mnist-elastic-worker-1", metav1.GetOptions{})
	// 	tolerations := []corev1.Toleration{toleration}
	// 	pod.Spec.Tolerations = tolerations
	// 	_, err = clientset.CoreV1().Pods("default").Update(context.TODO(), pod, metav1.UpdateOptions{})
	// 	return err
	// })
	// If modify existing toleration at runtime:
	// panic: Pod "tensorflow2-keras-mnist-elastic-worker-1" is invalid: spec.tolerations: Forbidden: existing toleration can not be modified except its tolerationSeconds

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		pod, err := clientset.CoreV1().Pods("default").Get(context.TODO(), "tensorflow2-keras-mnist-elastic-worker-1", metav1.GetOptions{})
		pod.SetNamespace("voda-scheduler")
		_, err = clientset.CoreV1().Pods("voda-scheduler").Update(context.TODO(), pod, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		panic(err.Error())
	}

	if err != nil {
		panic(err.Error())
	}
}
