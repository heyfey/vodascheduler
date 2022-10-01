package scheduler

import (
	kubeflowcommon "github.com/kubeflow/common/pkg/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
)

// https://github.com/kubeflow/mpi-operator/blob/6ee71d45dde0e71229b7fa91065e0c6bb503cd92/pkg/controllers/v1/mpi_job_controller_status.go#L77
func hasCondition(status kubeflowcommon.JobStatus, condType kubeflowcommon.JobConditionType) bool {
	for _, condition := range status.Conditions {
		if condition.Type == condType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// https://github.com/kubeflow/mpi-operator/blob/6ee71d45dde0e71229b7fa91065e0c6bb503cd92/pkg/controllers/v1/mpi_job_controller_status.go#L86
func isFinished(status kubeflowcommon.JobStatus) bool {
	return isSucceeded(status) || isFailed(status)
}

func isSucceeded(status kubeflowcommon.JobStatus) bool {
	return hasCondition(status, kubeflowcommon.JobSucceeded)
}

func isFailed(status kubeflowcommon.JobStatus) bool {
	return hasCondition(status, kubeflowcommon.JobFailed)
}
