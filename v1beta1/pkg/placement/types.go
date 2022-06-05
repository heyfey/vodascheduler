package placement

import (
	"github.com/heyfey/vodascheduler/config"
	corev1 "k8s.io/api/core/v1"
)

// request represents a single training job in JobScheduleResult.
type request struct {
	job        string
	numWorkers int
}

// nodeNumSlots represents a node and number of slots in the node that was
// allocated to a training job.
type nodeNumSlots struct {
	node     string
	numSlots int
}

// jobState represents state of a scheduled job.
type jobState struct {
	name       string
	numWorkers int
	// Note that:
	// 1. Sum of all numSlots in nodeNumSlotsList must equal to numWorkers.
	// 2. The order matters.
	nodeNumSlotsList []nodeNumSlots
}

// newJobState creates a new jobState.
func newJobState(name string) *jobState {
	j := &jobState{
		name:             name,
		numWorkers:       0,
		nodeNumSlotsList: []nodeNumSlots{},
	}
	return j
}

// nodeState represents state of a schedulable node.
type nodeState struct {
	name       string
	totalSlots int
	freeSlots  int
	// Jobs that was scheduled to this node and its number of workers on this node.
	// e.g. {"A": 4, "B": 4}.
	// Note that sum of numWorkers must <= totalSlots.
	jobNumWorkers map[string]int
	// Toleration to the node, will be added to pods scheduled to this node.
	toleration corev1.Toleration
}

// NewNodeState creates a new nodeState.
func NewNodeState(name string, slots int) *nodeState {
	n := &nodeState{
		name:          name,
		totalSlots:    slots,
		freeSlots:     slots,
		jobNumWorkers: make(map[string]int),
	}
	n.setToleration()
	return n
}

// setToleration sets the toleration of a schedulableNode by its name.
func (n *nodeState) setToleration() {
	toleration := corev1.Toleration{
		Key:      config.TaintKey,
		Operator: corev1.TolerationOpEqual,
		Value:    n.name,
		Effect:   corev1.TaintEffectNoExecute,
	}
	n.toleration = toleration
}

// rename updates the name of a schedulableNode.
func (n *nodeState) rename(name string) {
	n.name = name
	n.setToleration()
}
