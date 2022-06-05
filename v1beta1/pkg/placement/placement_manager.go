package placement

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"

	"github.com/heyfey/munkres"
	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

const gpuNameLabel = "vodascheduler/accelerator"

// Toleration to be added to the launcher pod.
var launcherToleration = corev1.Toleration{
	Key:      config.TaintKey,
	Operator: corev1.TolerationOpExists,
	Effect:   corev1.TaintEffectNoExecute,
}

type PlacementManager struct {
	SchedulerID  string
	kClient      kubeClient.Interface
	podInformer  cache.SharedIndexInformer
	nodeInformer cache.SharedIndexInformer

	// States of the placement manager, will be updated every time the placements
	// are adjust, should be protected by placementLock.
	// nodes and their states.
	nodeStates map[string]*nodeState
	// jobs and their states.
	jobStates map[string]*jobState
	// node for each pod.
	// e.g. {"A-worker-0": "gpu3", "A-worker-1": "gpu4", ...}
	podNodeName map[string]string
	// placementLock is used to protect states of the placement manager.
	placementLock sync.RWMutex

	StopCh chan struct{}

	metrics PlacementManagerMetrics
}

func discoverNodes(kClient *kubeClient.Clientset, gpuType string) (map[string]*nodeState, error) {
	availableNodes := make(map[string]*nodeState)

	labelSelector := gpuNameLabel + "=" + gpuType
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	nodes, err := kClient.CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		numGpus := countGPUs(node)
		availableNodes[node.Name] = NewNodeState(node.Name, numGpus)
	}
	return availableNodes, err
}

// NewPlacementManager creates a new placement manager.
func NewPlacementManager(id string, kConfig *rest.Config) (*PlacementManager, error) {
	kClient, err := kubeClient.NewForConfig(kConfig)
	if err != nil {
		return nil, err
	}

	// setup pod informer
	sharedInformers := informers.NewSharedInformerFactoryWithOptions(kClient, 0,
		informers.WithNamespace(config.Namespace))
	podListerInformer := sharedInformers.Core().V1().Pods()
	podInformer := podListerInformer.Informer()

	// setup node informer
	labelOptions := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = gpuNameLabel + "=" + id
	})
	factory := informers.NewSharedInformerFactoryWithOptions(kClient, 0, labelOptions)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	nodes, err := discoverNodes(kClient, id)
	if err != nil {
		return nil, err
	}

	pm := &PlacementManager{
		SchedulerID:   id,
		nodeStates:    nodes,
		jobStates:     map[string]*jobState{},
		podNodeName:   map[string]string{},
		placementLock: sync.RWMutex{},
		kClient:       kClient,
		podInformer:   podInformer,
		nodeInformer:  nodeInformer,
		StopCh:        make(chan struct{}),
	}
	pm.metrics = pm.initPlacementManagerMetrics()

	// setup informer callbacks
	pm.podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    pm.addPod,
			UpdateFunc: pm.updatePod,
		},
	)

	pm.nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    pm.addNode,
			UpdateFunc: pm.updateNode,
			DeleteFunc: pm.deleteNode,
		},
	)

	go pm.Run(pm.StopCh)

	return pm, nil
}

func (pm *PlacementManager) Run(stopCh <-chan struct{}) {
	klog.InfoS("Starting placement manager")
	defer klog.InfoS("Stopping placement manager")

	for _, n := range pm.nodeStates {
		klog.InfoS("Discovered node and its GPUs", "node", klog.KRef("", n.name),
			"numGpus", n.totalSlots)
	}
	// TODO(heyfey): defer handle crash

	go pm.podInformer.Run(stopCh)
	if !cache.WaitForCacheSync(
		stopCh,
		pm.podInformer.HasSynced) {
		err := errors.New("failed to WaitForCacheSync")
		klog.ErrorS(err, "Placement manager failed to WaitForCacheSync")
		klog.Flush()
		os.Exit(1)
	}

	<-stopCh
}

// addPod adds toleration to launcher and worker pod of MPIJobs.
func (pm *PlacementManager) addPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to add pod",
			"pod", klog.KObj(pod))
		return
	}
	klog.V(5).InfoS("Pod added", "pod", klog.KObj(pod))

	if isMPIJobLauncher(pod) && !hasToleration(pod, launcherToleration) {
		pm.addPodToleration(pod, launcherToleration)
	} else if isMPIJobWorker(pod) {
		pm.placementLock.RLock()
		nodeName, ok := pm.podNodeName[pod.GetName()]
		if ok {
			t := pm.nodeStates[nodeName].toleration
			if !hasToleration(pod, t) {
				pm.addPodToleration(pod, t)
			}
		} else {
			klog.Flush()
			panic(errors.New("could not find pod in podNodeName table"))
		}
		pm.placementLock.RUnlock()
	}
}

func (pm *PlacementManager) updatePod(oldObj interface{}, newObj interface{}) {
	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to update pod",
			"pod", klog.KObj(oldPod))
		return
	}
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to update pod",
			"pod", klog.KObj(newPod))
		return
	}
	// Informer may deliver an Update event with UID changed if a delete is
	// immediately followed by a create, so manually decompose it.
	if oldPod.UID != newPod.UID {
		pm.addPod(newObj)
		return
	}
}

// addPodToleration appends the toleration to pod and updates it.
func (pm *PlacementManager) addPodToleration(pod *corev1.Pod, toleration corev1.Toleration) {
	klog.V(4).InfoS("Adding toleration to pod", "pod", klog.KObj(pod), "toleration", toleration)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		pod, err := pm.kClient.CoreV1().Pods(config.Namespace).Get(context.TODO(), pod.GetName(), metav1.GetOptions{})
		pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)
		_, err = pm.kClient.CoreV1().Pods(config.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		klog.ErrorS(err, "Failed to update pod", "pod", klog.KObj(pod)) // TODO: error handling
	}
}

func (pm *PlacementManager) addNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to add Node",
			"node", klog.KObj(node))
		return
	}

	pm.placementLock.Lock()
	defer pm.placementLock.Unlock()

	numGpus := countGPUs(*node)
	pm.nodeStates[node.GetName()] = NewNodeState(node.GetName(), numGpus)

	klog.InfoS("Node added", "node", klog.KObj(node), "numGpus", numGpus)
}

func (pm *PlacementManager) updateNode(oldObj interface{}, newObj interface{}) {
	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to update node",
			"node", klog.KObj(oldNode))
		return
	}
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to update node",
			"node", klog.KObj(newNode))
		return
	}
	// Informer may deliver an Update event with UID changed if a delete is
	// immediately followed by a create, so manually decompose it.
	if oldNode.UID != newNode.UID {
		pm.deleteNode(oldObj)
		pm.addNode(newObj)
	}
}

func (pm *PlacementManager) deleteNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"),
			"Failed to delete Node", "node", klog.KObj(node))
		return
	}

	pm.placementLock.Lock()
	defer pm.placementLock.Unlock()

	for job, workers := range pm.nodeStates[node.GetName()].jobNumWorkers {
		pm.jobStates[job].numWorkers -= workers
		for _, nSlots := range pm.jobStates[job].nodeNumSlotsList {
			if nSlots.node == node.GetName() {
				nSlots.numSlots = 0
				break
			}
		}
	}
	delete(pm.nodeStates, node.GetName())
	klog.InfoS("Node deleted", "node", klog.KObj(node))
}

func (pm *PlacementManager) Place(jobRequests types.JobScheduleResult) {
	klog.InfoS("Started placement adjustment")
	defer klog.InfoS("Finished placement adjustment")

	pm.placementLock.Lock()
	timer := prometheus.NewTimer(pm.metrics.placementAlgoDuration)

	/***** Placement algorithm begin *****/
	pm.releaseSlots(jobRequests)

	// construct empty nodes from nodeStates
	schedulableNodesList := make([]*nodeState, 0, len(pm.nodeStates))
	for _, n := range pm.nodeStates {
		schedulableNodesList = append(schedulableNodesList, NewNodeState("TBD", n.totalSlots))
	}
	pm.bestFit(jobRequests, schedulableNodesList)

	pm.bindNodes(schedulableNodesList)
	pm.updateJobStates()
	deletingPodList := pm.updatePodNodeName()
	/***** Placement algorithm end *****/

	timer.ObserveDuration()
	pm.placementLock.Unlock()

	pm.deletePods(deletingPodList)
}

// releaseSlots releases spare slots for the jobs had been scaled down or
// terminated. Releasing slots should be done in both jobStates and nodeStates
// becauese they represents the same states only in different perspectives.
func (pm *PlacementManager) releaseSlots(jobRequests types.JobScheduleResult) {
	klog.V(4).InfoS("Releasing slots", "jobs", pm.jobStates, "nodes", pm.nodeStates)
	defer klog.V(4).InfoS("Released slots", "jobs", pm.jobStates, "nodes", pm.nodeStates)

	for _, job := range pm.jobStates {
		numWorkers, ok := jobRequests[job.name]
		if !ok {
			// The training job wasn't scheduled, which has been terminated,
			// thus release all slots of the job.
			for _, nSlots := range job.nodeNumSlotsList {
				klog.V(5).InfoS("Released slots", "job", job.name, "node", nSlots.node, "slots", nSlots.numSlots)

				node, ok := pm.nodeStates[nSlots.node]
				if ok {
					node.freeSlots += nSlots.numSlots
					delete(node.jobNumWorkers, job.name)
				} else {
					// node has been deleted, do nothing
					// nSlots.slots == 0
				}
			}
			job.nodeNumSlotsList = job.nodeNumSlotsList[:0]

		} else if numWorkers < job.numWorkers {
			// the training job has been scaled down
			toRelease := job.numWorkers - numWorkers // total number of slots of a job to be released

			// Keep releasing slots from the last allocated node of the job,
			// this is because MPI Operator always deletes worker pods from max
			// index to 0 when scaling down a job.
			for toRelease > 0 {
				last := len(job.nodeNumSlotsList) - 1
				lastNodeNumSlots := job.nodeNumSlotsList[last]
				node, ok := pm.nodeStates[lastNodeNumSlots.node]

				if lastNodeNumSlots.numSlots >= toRelease {
					// the release can be done by release some slots in this node
					klog.V(5).InfoS("Released slots", "job", job.name, "node", node.name, "slots")

					lastNodeNumSlots.numSlots -= toRelease
					node.freeSlots += toRelease
					node.jobNumWorkers[job.name] -= toRelease
					toRelease = 0
				} else {
					// It is not enough even if we release all slots of the job
					// in this node. This case includes lastNodeNumSlots.slots == 0,
					// which means the node has been deleted.
					klog.V(5).InfoS("Released slots", "job", job.name, "node", node.name,
						"slots", lastNodeNumSlots.numSlots)

					toRelease -= lastNodeNumSlots.numSlots
					lastNodeNumSlots.numSlots = 0
					if ok {
						node.freeSlots += node.jobNumWorkers[job.name]
						node.jobNumWorkers[job.name] = 0
					} else {
						// Node has been deleted.
						// lastNodeNumSlots.slots should equal 0 in this case
					}
				}
				job.nodeNumSlotsList[last] = lastNodeNumSlots

				// The following two statements should always be true at the same time.
				// remove the node from the job if the job has 0 slot in the node
				if job.nodeNumSlotsList[last].numSlots == 0 {
					job.nodeNumSlotsList = job.nodeNumSlotsList[:last]
				}
				// remove the job from the node if the node gives 0 slot to the job
				if ok && node.jobNumWorkers[job.name] == 0 {
					delete(node.jobNumWorkers, job.name)
				}
			}
		}
	}
}

// bestFit sorts training jobs by number of workers requested in descending
// order, then binds training jobs to nodes using best-fit algorithm.
func (pm *PlacementManager) bestFit(jobRequests types.JobScheduleResult, nodeList []*nodeState) {
	requests := make([]request, len(jobRequests))
	for job, n := range jobRequests {
		requests = append(requests, request{job: job, numWorkers: n})
	}

	// sort the list by number of workers requested in descending order
	sort.SliceStable(requests, func(i, j int) bool {
		return requests[i].numWorkers > requests[j].numWorkers
	})

	totalClusterSlots := 0
	for _, node := range nodeList {
		totalClusterSlots += node.totalSlots
	}

	// calculate how many jobs require cross-node communication
	crossNode := 0

	defer klog.V(4).InfoS("Found best-fit", "nodes", nodeList, "requests", requests, "numJobCrossNode", crossNode)

	// start finding best-fit for each request
	for _, r := range requests {
		requested := r.numWorkers
		for requested > 0 {
			// Cluster info could be inconsistent between scheduler and
			// placement manager in some cases, for example:
			//      1. Node informer in scheduler noticed a node addition.
			//      2. Scheduler register the added node, trigger rescheduling,
			//         and ask for new placements for new scheduling results.
			//      3. However, node informer in placement manager hasn't noticed
			//         the same node addition.
			// Placement manager decides to tolerates this inconsistency, rather
			// than pursue strong consistency between scheduler and placement
			// manager. Even though the inconsistency might cause some resources
			// under-utilization between two rescheduling periods, it shouldn't
			// crash the whole system.
			if totalClusterSlots == 0 { // total requested may not always equal to totalClusterSlots
				return
			}

			bestIdx := -1
			maxIdx := 0
			for i, node := range nodeList {
				// find the bestfit
				if node.freeSlots >= requested {
					if bestIdx == -1 {
						bestIdx = i
					} else if nodeList[bestIdx].freeSlots > node.freeSlots {
						bestIdx = i
					}
				}
				// also find the node with max free slots
				if nodeList[maxIdx].freeSlots < node.freeSlots {
					maxIdx = i
				}
			}
			if bestIdx == -1 { // best fit not found, use the node with max free slots
				nodeList[maxIdx].jobNumWorkers[r.job] = nodeList[maxIdx].freeSlots
				requested -= nodeList[maxIdx].freeSlots
				totalClusterSlots -= nodeList[maxIdx].freeSlots
				nodeList[maxIdx].freeSlots = 0
				crossNode++
			} else {
				nodeList[bestIdx].jobNumWorkers[r.job] = r.numWorkers
				requested -= r.numWorkers
				nodeList[bestIdx].freeSlots -= r.numWorkers
				totalClusterSlots -= r.numWorkers
			}
		}
	}
}

// bindNodes constructs new nodeStates and replace the original ones by replacing
// each node in the original nodeStates with one of the node selected from
// provided nodes.
func (pm *PlacementManager) bindNodes(nodeList []*nodeState) {
	size := len(pm.nodeStates)
	currentNodeList := make([]*nodeState, 0, size)
	for _, node := range pm.nodeStates {
		currentNodeList = append(currentNodeList, node)
	}

	scoringMatrix := make([]int64, 0, size*size)
	for _, node := range nodeList {
		scoringMatrix = append(scoringMatrix, pm.scoreCandidates(node, currentNodeList)...)
	}
	klog.V(5).InfoS("Scored all nodes", "scoringMatrix", scoringMatrix)

	m := munkres.NewMatrix(size)
	m.A = scoringMatrix
	result := munkres.ComputeMunkresMax(m)
	totalScore := int64(0)
	for _, rowCol := range result {
		nodeList[rowCol.Row].rename(currentNodeList[rowCol.Col].name)
		totalScore += scoringMatrix[size*rowCol.Row+rowCol.Col]
	}

	newNodeStates := make(map[string]*nodeState)
	for _, node := range nodeList {
		newNodeStates[node.name] = node
	}
	klog.V(4).InfoS("Updated node states", "oldStates", pm.nodeStates,
		"newStates", newNodeStates, "score", totalScore)

	pm.nodeStates = newNodeStates
}

// scoreCandidates scores all the candidate nodes for a node.
func (pm *PlacementManager) scoreCandidates(position *nodeState, candidateList []*nodeState) []int64 {
	scores := make([]int64, len(candidateList))
	for i, candidate := range candidateList {
		scores[i] = pm.score(position, candidate)
	}
	return scores
}

// score calculates overlap of job:workers in two nodes.
func (pm *PlacementManager) score(position *nodeState, candidate *nodeState) int64 {
	score := 0
	for job, workers := range position.jobNumWorkers {
		if candidate.jobNumWorkers[job] <= workers {
			score += candidate.jobNumWorkers[job]
		} else {
			score += position.jobNumWorkers[job]
		}
	}
	return int64(score)
}

// updateJobStates constructs new JobStates from nodeStates and replaces
// the original one.
func (pm *PlacementManager) updateJobStates() {
	newJobStates := make(map[string]*jobState)
	for _, node := range pm.nodeStates {
		for jobName, workers := range node.jobNumWorkers {
			_, ok := newJobStates[jobName]
			if !ok {
				newJobStates[jobName] = newJobState(jobName)
			}
			newJobStates[jobName].nodeNumSlotsList = append(newJobStates[jobName].nodeNumSlotsList,
				nodeNumSlots{node: node.name, numSlots: workers})
			newJobStates[jobName].numWorkers += workers

			// TODO: Considering the order in nodeNumSlotsList
		}
	}
	klog.V(4).InfoS("Updated job states", "oldStates", pm.jobStates, "newStates", newJobStates)

	pm.jobStates = newJobStates
}

// updatePodNodeName
// 1. constructs a new podNodeName from jobStates and replaces the original one.
// 2. returns a list of pods whose node was changed thus need migration.
func (pm *PlacementManager) updatePodNodeName() []string {
	newPodNodeName := make(map[string]string)

	deletingPodList := make([]string, 0) // pods to be deleted to perform migrations
	deletedWorkers := 0
	deletedLaunchers := 0

	for _, job := range pm.jobStates {
		idx := 0
		deleted := 0
		for _, nSlots := range job.nodeNumSlotsList {
			for i := 0; i < nSlots.numSlots; i++ {
				podName := getWorkerPodName(job.name, idx)

				klog.V(5).InfoS("Updating podNodeName table",
					"pod", klog.KRef(config.Namespace, podName), "nodeName", nSlots.node)

				// determine if the pod need to be deleted
				oldNode, ok := pm.podNodeName[podName]
				if ok && nSlots.node != oldNode {
					deletingPodList = append(deletingPodList, podName)
					deleted++
					deletedWorkers++

					klog.V(5).InfoS("Found worker pod need migration",
						klog.KRef(config.Namespace, podName), "fromNode", oldNode, "toNode", nSlots.node)
				}
				newPodNodeName[podName] = nSlots.node
				idx++
			}
		}
		if deleted == job.numWorkers {
			deletingPodList = append(deletingPodList, getLauncherPodName(job.name))
			deletedLaunchers++
		}
	}
	klog.V(4).InfoS("Updated podNodeName table", "oldTable", pm.podNodeName, "newTable", newPodNodeName,
		"numWorkersToDelete", deletedWorkers, "numLaunchersToDelete", deletedLaunchers)

	pm.podNodeName = newPodNodeName

	return deletingPodList
}

// deletePods deletes pods.
// The deleted pods will be re-created by the MPIJob controller, and tolerations
// will be added to pods by the informer callbacks.
func (pm *PlacementManager) deletePods(podList []string) {
	for _, podName := range podList {
		err := pm.kClient.CoreV1().Pods(config.Namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to delete pod for migration",
				"pod", klog.KRef(config.Namespace, podName))
		} else {
			klog.V(4).InfoS("Deleted pod that need migration",
				"pod", klog.KRef(config.Namespace, podName))
		}
	}
}
