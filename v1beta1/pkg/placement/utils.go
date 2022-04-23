package placement

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	// should be the same as:
	// https://github.com/kubeflow/mpi-operator/blob/master/pkg/controllers/v1/mpi_job_controller.go#L61
	// permalink:
	// https://github.com/kubeflow/mpi-operator/blob/30cdf43f933157111100b9a4e409b7b78d2c09cc/pkg/controllers/v1/mpi_job_controller.go#L61
	launcher         = "launcher"
	worker           = "worker"
	labelMPIRoleType = "mpi-job-role"
)

func getLauncherPodName(name string) podName {
	return podName(fmt.Sprintf("%s-%s", name, launcher))
}

func getWorkerPodName(name string, idx int) podName {
	return podName(fmt.Sprintf("%s-%s-%d", name, worker, idx))
}

func isMPIJobLauncher(pod *corev1.Pod) bool {
	return pod.GetLabels()[labelMPIRoleType] == launcher
}

func isMPIJobWorker(pod *corev1.Pod) bool {
	return pod.GetLabels()[labelMPIRoleType] == worker
}

func hasToleration(pod *corev1.Pod, toleration corev1.Toleration) bool {
	for _, t := range pod.Spec.Tolerations {
		if t.MatchToleration(&toleration) {
			return true
		}
	}
	return false
}

func countGPUs(node corev1.Node) int {
	gpus := node.Status.Capacity["nvidia.com/gpu"]
	return int(gpus.Value())
}
