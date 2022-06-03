package scheduler

import (
	"fmt"
	"testing"
	"time"

	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	mpijobfake "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/fake"
	mpiinformers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	mpiClient  *mpijobfake.Clientset
	kubeClient *k8sfake.Clientset

	// Objects to put in the store.
	podLister    []*corev1.Pod
	mpiJobLister []*kubeflowv1.MPIJob

	// Actions expected to happen on the client.
	kubeActions []core.Action
	actions     []core.Action

	// Objects from here are pre-loaded into NewSimpleFake.
	kubeObjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeObjects = []runtime.Object{}
	return f
}

func (f *fixture) newScheduler() *Scheduler {
	f.mpiClient = mpijobfake.NewSimpleClientset(f.objects...)
	f.kubeClient = k8sfake.NewSimpleClientset(f.kubeObjects...)

	i := mpiinformers.NewSharedInformerFactory(f.mpiClient, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeClient, noResyncPeriodFunc())

	s, _ := NewScheduler("test", nil, nil, databaseNameRunningJobs)

	for _, pod := range f.podLister {
		err := k8sI.Core().V1().Pods().Informer().GetIndexer().Add(pod)
		if err != nil {
			fmt.Println("Failed to create pod")
		}
	}

	for _, mpiJob := range f.mpiJobLister {
		err := i.Kubeflow().V1().MPIJobs().Informer().GetIndexer().Add(mpiJob)
		if err != nil {
			fmt.Println("Failed to create mpijob")
		}
	}

	return s
}

func (f *fixture) setUpMPIJob(mpiJob *kubeflowv1.MPIJob) {
	f.mpiJobLister = append(f.mpiJobLister, mpiJob)
	f.objects = append(f.objects, mpiJob)
}

func TestDummy(t *testing.T) {

}

func TestCompareResults(t *testing.T) {

}
