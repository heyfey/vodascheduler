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

// Toleration to be added to the launcher pod.
var launcherToleration = corev1.Toleration{
	Key:      config.TaintKey,
	Operator: corev1.TolerationOpExists,
	Effect:   corev1.TaintEffectNoExecute,
}

type PlacementManager struct {
	SchedulerID string
	kClient     kubeClient.Interface
	podInformer cache.SharedIndexInformer

	// States of the placement manager, will be updated every time the placements
	// are adjust, should be protected by placementLock.
	// nodes and their states.
	nodeStates map[nodeName]*nodeState
	// jobs and their states.
	jobStates map[string]*jobState
	// node for each pod.
	// e.g. {"A-worker-0": "gpu3", "A-worker-1": "gpu4", ...}
	podNodeName map[podName]nodeName
	// placementLock is used to protect states of the placement manager.
	placementLock sync.RWMutex

	StopCh chan struct{}

	metrics PlacementManagerMetrics
}

func discoverNodes(config *rest.Config) (map[nodeName]*nodeState, error) {
	availableNodes := make(map[nodeName]*nodeState)

	clientset, err := kubeClient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clusterNodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range clusterNodes.Items {
		gpus := node.Status.Capacity["nvidia.com/gpu"]
		availableNodes[nodeName(node.Name)] = NewNodeState(nodeName(node.Name), int(gpus.Value()))
	}
	return availableNodes, err
}

// NewPlacementManager creates a new placement manager.
func NewPlacementManager(id string, kConfig *rest.Config) (*PlacementManager, error) {
	kClient, err := kubeClient.NewForConfig(kConfig)
	if err != nil {
		return nil, err
	}
	sharedInformers := informers.NewSharedInformerFactory(kClient, 0)
	podListerInformer := sharedInformers.Core().V1().Pods()
	podInformer := podListerInformer.Informer()

	nodes, err := discoverNodes(kConfig)
	if err != nil {
		return nil, err
	}

	pm := &PlacementManager{
		SchedulerID:   id,
		nodeStates:    nodes,
		jobStates:     map[string]*jobState{},
		podNodeName:   map[podName]nodeName{},
		placementLock: sync.RWMutex{},
		kClient:       kClient,
		podInformer:   podInformer,
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

	go pm.Run(pm.StopCh)

	return pm, nil
}

func (pm *PlacementManager) Run(stopCh <-chan struct{}) {
	klog.InfoS("Starting placement manager", "scheduler", pm.SchedulerID)
	for _, n := range pm.nodeStates {
		klog.InfoS("Discovered nodes and their GPUs", "scheduler", pm.SchedulerID, "nodes", n.name, "num_gpus", n.totalSlots)
	}
	defer klog.InfoS("Stopping placement manager", "scheduler", pm.SchedulerID)
	// TODO(heyfey): defer handle crash

	go pm.podInformer.Run(stopCh)
	if !cache.WaitForCacheSync(
		stopCh,
		pm.podInformer.HasSynced) {
		err := errors.New("failed to WaitForCacheSync")
		klog.ErrorS(err, "Placement manager failed to WaitForCacheSync", "scheduler", pm.SchedulerID)
		klog.Flush()
		os.Exit(1)
	}

	<-stopCh
}

// addPod adds toleration to launcher and worker pod of MPIJobs.
func (pm *PlacementManager) addPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to update pod", "pod", klog.KObj(pod),
			"scheduler", pm.SchedulerID)
		return
	}
	klog.V(5).InfoS("Pod added", "pod", klog.KObj(pod), "scheduler", pm.SchedulerID)

	if isMPIJobLauncher(pod) && !hasToleration(pod, launcherToleration) {
		pm.addPodToleration(pod, launcherToleration)
	} else if isMPIJobWorker(pod) {
		pm.placementLock.RLock()
		nodeName, ok := pm.podNodeName[podName(pod.GetName())]
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
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to update pod", "pod", klog.KObj(oldPod),
			"scheduler", pm.SchedulerID)
		return
	}
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(errors.New("unexpected pod type"), "Failed to update pod", "pod", klog.KObj(newPod),
			"scheduler", pm.SchedulerID)
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
	klog.V(4).InfoS("Adding toleration to pod", "pod", klog.KObj(pod), "toleration", toleration,
		"scheduler", pm.SchedulerID)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		pod, err := pm.kClient.CoreV1().Pods("default").Get(context.TODO(), pod.GetName(), metav1.GetOptions{}) // TODO: set namespace
		pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)
		_, err = pm.kClient.CoreV1().Pods("default").Update(context.TODO(), pod, metav1.UpdateOptions{}) // TODO: set namespace
		return err
	})
	if err != nil {
		klog.ErrorS(err, "Failed to update pod", "pod", klog.KObj(pod), "scheduler", pm.SchedulerID) // TODO: error handling
	}
}

func (pm *PlacementManager) Place(jobRequests types.JobScheduleResult) {
	klog.InfoS("Started placement adjustment", "scheduler", pm.SchedulerID)
	defer klog.InfoS("Finished placement adjustment", "scheduler", pm.SchedulerID)

	pm.placementLock.Lock()
	timer := prometheus.NewTimer(pm.metrics.placementAlgoDuration)

	// Starting placement algorithm
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
	// Placement algorithm ended

	timer.ObserveDuration()
	pm.placementLock.Unlock()

	pm.deletePods(deletingPodList)
}

// releaseSlots releases spare slots for the jobs had been scaled down or
// terminated. Releasing slots should be done in both jobStates and nodeStates
// becauese they represents the same states only in different perspectives.
func (pm *PlacementManager) releaseSlots(jobRequests types.JobScheduleResult) {
	klog.V(4).InfoS("Releasing slots", "jobs", pm.jobStates, "nodes", pm.nodeStates, "scheduler", pm.SchedulerID)
	defer klog.V(4).InfoS("Released slots", "jobs", pm.jobStates, "nodes", pm.nodeStates, "scheduler", pm.SchedulerID)

	for _, job := range pm.jobStates {
		workers, ok := jobRequests[job.name]
		if !ok {
			// The training job wasn't scheduled, which has been terminated,
			// thus release all slots of the job.
			for _, hs := range job.nodeSlotsList {
				klog.V(5).InfoS("Released slots", "job", job.name, "node", hs.node, "slots", hs.slots,
					"scheduler", pm.SchedulerID)

				pm.nodeStates[hs.node].freeSlots += hs.slots
				delete(pm.nodeStates[hs.node].jobWorkers, job.name)
			}
			job.nodeSlotsList = job.nodeSlotsList[:0]

		} else if workers < job.workers {
			// the training job has been scaled down
			toRelease := job.workers - workers // how many slots need to be released
			// keep releasing slots from the last allocated node of the job
			for toRelease > 0 {
				last := len(job.nodeSlotsList) - 1
				node := pm.nodeStates[job.nodeSlotsList[last].node]

				if job.nodeSlotsList[last].slots >= toRelease {
					// the release can be done by release some slots in this node
					klog.V(5).InfoS("Released slots", "job", job.name, "node", node.name, "slots",
						toRelease, "scheduler", pm.SchedulerID)

					job.nodeSlotsList[last].slots -= toRelease
					node.freeSlots += toRelease
					node.jobWorkers[job.name] -= toRelease
					toRelease = 0
				} else {
					// it is not enough even if we release all slots of the job
					// in this node
					klog.V(5).InfoS("Released slots", "job", job.name, "node", node.name,
						"slots", job.nodeSlotsList[last].slots, "scheduler", pm.SchedulerID)

					toRelease -= job.nodeSlotsList[last].slots
					job.nodeSlotsList[last].slots = 0
					node.freeSlots += node.jobWorkers[job.name]
					node.jobWorkers[job.name] = 0
				}

				// The following two statements should always be true at the same time.
				// remove the node from the job if the job has 0 slot in the node
				if job.nodeSlotsList[last].slots == 0 {
					job.nodeSlotsList = job.nodeSlotsList[:last]
				}
				// remove the job from the node if the node gives 0 slot to the job
				if node.jobWorkers[job.name] == 0 {
					delete(node.jobWorkers, job.name)
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
		requests = append(requests, request{job: job, workers: n})
	}

	// sort the list by number of workers requested in descending order
	sort.SliceStable(requests, func(i, j int) bool {
		return requests[i].workers > requests[j].workers
	})

	crossNode := 0

	defer klog.V(4).InfoS("Found best-fit", "nodes", nodeList, "requests", requests, "numJobCrossNode", crossNode,
		"scheduler", pm.SchedulerID)

	// start finding best-fit for each request
	for _, r := range requests {
		requested := r.workers
		for requested > 0 {
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
				nodeList[maxIdx].jobWorkers[r.job] = nodeList[maxIdx].freeSlots
				requested -= nodeList[maxIdx].freeSlots
				nodeList[maxIdx].freeSlots = 0
				crossNode++
			} else {
				nodeList[bestIdx].jobWorkers[r.job] = r.workers
				requested -= r.workers
				nodeList[bestIdx].freeSlots -= r.workers
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
	klog.V(5).InfoS("Scored all nodes", "scoringMatrix", scoringMatrix, "scheduler", pm.SchedulerID)

	m := munkres.NewMatrix(size)
	m.A = scoringMatrix
	result := munkres.ComputeMunkresMax(m)
	totalScore := int64(0)
	for _, rowCol := range result {
		nodeList[rowCol.Row].rename(currentNodeList[rowCol.Col].name)
		totalScore += scoringMatrix[size*rowCol.Row+rowCol.Col]
	}

	newNodeStates := make(map[nodeName]*nodeState)
	for _, node := range nodeList {
		newNodeStates[node.name] = node
	}
	klog.V(4).InfoS("Updated node states", "oldStates", pm.nodeStates, "newStates", newNodeStates, "score", totalScore,
		"scheduler", pm.SchedulerID)

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
	for job, workers := range position.jobWorkers {
		if candidate.jobWorkers[job] <= workers {
			score += candidate.jobWorkers[job]
		} else {
			score += position.jobWorkers[job]
		}
	}
	return int64(score)
}

// updateJobStates constructs new JobStates according to nodeStates and replaces
// the original ones.
func (pm *PlacementManager) updateJobStates() {
	newJobStates := make(map[string]*jobState)
	for _, node := range pm.nodeStates {
		for jobName, workers := range node.jobWorkers {
			_, ok := newJobStates[jobName]
			if !ok {
				newJobStates[jobName] = newJobState(jobName)
			}
			newJobStates[jobName].nodeSlotsList = append(newJobStates[jobName].nodeSlotsList,
				nodeSlots{node: node.name, slots: workers})
			newJobStates[jobName].workers += workers

			// TODO: Considering the order in nodeSlotsList
		}
	}
	klog.V(4).InfoS("Updated job states", "oldStates", pm.jobStates, "newStates", newJobStates,
		"scheduler", pm.SchedulerID)

	pm.jobStates = newJobStates
}

// updatePodNodeName 1. constructs a new podNodeName according to jobStates and
// replaces the original one. 2. returns a list of pods whose node was changed
// thus need migration.
func (pm *PlacementManager) updatePodNodeName() []podName {
	newPodNodeName := make(map[podName]nodeName)

	deletingPodList := make([]podName, 0) // pods to be deleted to perform migrations
	deletedWorkers := 0
	deletedLaunchers := 0

	for _, job := range pm.jobStates {
		idx := 0
		deleted := 0
		for _, hs := range job.nodeSlotsList {
			for i := 0; i < hs.slots; i++ {
				pod := getWorkerPodName(job.name, idx)

				klog.V(5).InfoS("Updating podNodeName table", "pod", klog.KRef("default", string(pod)),
					"nodeName", hs.node, "scheduler", pm.SchedulerID) // TODO(heyfey): namespace

				// determine if the pod need to be deleted
				oldNode, ok := pm.podNodeName[pod]
				if ok && hs.node != oldNode {
					deletingPodList = append(deletingPodList, pod)
					deleted++
					deletedWorkers++

					klog.V(5).InfoS("Found worker pod need migration", klog.KRef("default", string(pod)),
						"fromNode", oldNode, "toNode", hs.node, "scheduler", pm.SchedulerID) // TODO(heyfey): namespace
				}
				newPodNodeName[pod] = hs.node
				idx++
			}
		}
		if deleted == job.workers {
			deletingPodList = append(deletingPodList, getLauncherPodName(job.name))
			deletedLaunchers++
		}
	}
	klog.V(4).InfoS("Updated podNodeName table", "oldTable", pm.podNodeName, "newTable", newPodNodeName,
		"numWorkersToDelete", deletedWorkers, "numLaunchersToDelete", deletedLaunchers, "scheduler", pm.SchedulerID)

	pm.podNodeName = newPodNodeName

	return deletingPodList
}

// deletePods deletes pods.
// The deleted pods will be re-created by the MPIJob controller, and tolerations
// will be added to pods by the informer callbacks of the placement manager.
func (pm *PlacementManager) deletePods(podList []podName) {
	for _, pod := range podList {
		err := pm.kClient.CoreV1().Pods("default").Delete(context.TODO(), string(pod), metav1.DeleteOptions{}) // TODO: set namespace
		if err != nil {
			klog.ErrorS(err, "Failed to delete pod for migration", klog.KRef("default", string(pod)),
				"scheduler", pm.SchedulerID) // TODO: error handling
		} else {
			klog.V(4).InfoS("Deleted pod that need migration", "pod", klog.KRef("default", string(pod)),
				"scheduler", pm.SchedulerID) // TODO(heyfey): namespace
		}
	}
}
