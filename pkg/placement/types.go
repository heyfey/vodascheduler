package placement

import (
	"github.com/heyfey/vodascheduler/config"
	corev1 "k8s.io/api/core/v1"
)

// request represents a single training job in JobScheduleResult.
type request struct {
	job     string
	workers int
}

type (
	nodeName string
	podName  string
	// jobName  string
)

// nodeSlots represents a node and number of slots in the node that was
// allocated to a training job.
type nodeSlots struct {
	node  nodeName
	slots int
}

// nodeState represents state of a scheduled job.
type jobState struct {
	name    string
	workers int
	// Note that:
	// 1. The sum of slots must equal to workers.
	// 2. The order matters.
	nodeSlotsList []nodeSlots
}

// newJobState creates a new jobState.
func newJobState(name string) *jobState {
	j := &jobState{
		name:          name,
		workers:       0,
		nodeSlotsList: []nodeSlots{},
	}
	return j
}

// nodeState represents state of a schedulable node.
type nodeState struct {
	name       nodeName
	totalSlots int
	freeSlots  int
	// Jobs that was scheduled to this node and its number of workers on this node.
	// e.g. {"A": 4, "B": 4}.
	// Note that sum of workers must <= totalSlots.
	jobWorkers map[string]int
	// Toleration to the node, will be added to pods scheduled to this node.
	toleration corev1.Toleration
}

// NewNodeState creates a new nodeState.
func NewNodeState(name nodeName, slots int) *nodeState {
	n := &nodeState{
		name:       name,
		totalSlots: slots,
		freeSlots:  slots,
		jobWorkers: make(map[string]int),
	}
	n.setToleration()
	return n
}

// setToleration sets the toleration of a schedulableNode by its name.
func (n *nodeState) setToleration() {
	toleration := corev1.Toleration{
		Key:      config.TaintKey,
		Operator: corev1.TolerationOpEqual,
		Value:    string(n.name),
		Effect:   corev1.TaintEffectNoExecute,
	}
	n.toleration = toleration
}

// rename updates the name of a schedulableNode.
func (n *nodeState) rename(name nodeName) {
	n.name = name
	n.setToleration()
}
