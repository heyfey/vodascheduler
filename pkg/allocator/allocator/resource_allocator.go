package allocator

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/heyfey/vodascheduler/pkg/algorithm"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"k8s.io/klog/v2"
)

const (
	entryPoint          = "/allocation"
	databaseNameJobInfo = "job_info"
)

type ResourceAllocator struct {
	session *mgo.Session
	Router  *mux.Router
	Metrics ResourceAllocatorMetrics
}

func NewResourceAllocator() *ResourceAllocator {
	ra := &ResourceAllocator{
		session: mongo.ConnectMongo(),
		Router:  mux.NewRouter(),
	}
	ra.initRoutes()
	ra.initResourceAllocatorMetrics()
	return ra
}

func (ra *ResourceAllocator) initRoutes() {
	ra.Router.HandleFunc(entryPoint, ra.allocateResourceHandler()).Methods("POST")
	ra.Router.Handle("/metrics", promhttp.Handler())
}

func (ra *ResourceAllocator) allocateResourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		klog.InfoS("Endpoint hit", "endpoint", entryPoint)

		var newReq AllocationRequest
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err)
			return
		}

		err = json.Unmarshal(reqBody, &newReq)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err)
			return
		}

		allocation, err := ra.allocateResource(&newReq)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err)
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(allocation)
		}
	}
}

func (ra *ResourceAllocator) allocateResource(req *AllocationRequest) (types.JobScheduleResult, error) {
	klog.InfoS("Allocating resource",
		"algorithm", req.AlgorithmName, "numGpus", req.NumGpu, "scheduler", req.SchedulerID)
	defer klog.V(4).InfoS("Finished allocating resource",
		"algorithm", req.AlgorithmName, "numGpus", req.NumGpu, "scheduler", req.SchedulerID)

	numReadyJobs := len(req.ReadyJobs)
	ra.Metrics.numReadyJobs.Observe(float64(numReadyJobs))
	ra.Metrics.numReadyJobsLabeled.WithLabelValues(req.AlgorithmName).Observe(float64(numReadyJobs))

	ra.Metrics.numGpus.Observe(float64(req.NumGpu))
	ra.Metrics.numGpusLabeled.WithLabelValues(req.AlgorithmName).Observe(float64(req.NumGpu))

	algorithm, err := algorithm.NewAlgorithmFactory(req.AlgorithmName, req.SchedulerID)
	if err != nil {
		klog.ErrorS(err, "Failed to create algorithm",
			"algorithm", req.AlgorithmName, "scheduler", req.SchedulerID)
		return nil, err
	}

	// 1. get all job info from DB if needed
	if algorithm.NeedJobInfo() {
		timer := prometheus.NewTimer(ra.Metrics.accessDBDuration)
		ra.getJobsInfo(req)
		timer.ObserveDuration()
	}

	// 2. allocate resources
	timer := prometheus.NewTimer(ra.Metrics.schedulingAlgorithmDuration)
	timer2 := prometheus.NewTimer(ra.Metrics.schedulingAlgorithmDurationLabeled.WithLabelValues(req.AlgorithmName))
	allocation := algorithm.Schedule(req.ReadyJobs, req.NumGpu)
	go timer.ObserveDuration()
	timer2.ObserveDuration()

	return allocation, nil
}

// getJobsInfo finds information of all training jobs in mongodb and update the
// training jobs' info.
func (ra *ResourceAllocator) getJobsInfo(req *AllocationRequest) {
	sess := ra.session.Clone()
	defer sess.Close()

	klog.V(4).InfoS("Getting all jobs info")

	for _, job := range req.ReadyJobs {
		trainingJobInfo := trainingjob.NewBaseJobInfo(job.Name, job.Category, job.GpuType)
		trainingJobInfoMongo := mongo.TrainingJobInfo{}
		err := sess.DB(databaseNameJobInfo).C(job.Category).Find(bson.M{"name": job.Name}).One(&trainingJobInfoMongo)
		if err != nil {
			klog.ErrorS(err, "Failed to find job info, using basic info",
				"db", databaseNameJobInfo, "job", job.Name, "category", job.Category)
		} else {
			klog.V(5).InfoS("Got job info", "job", job)
			trainingJobInfo.EstimatedRemainningTimeSec = trainingJobInfoMongo.EstimatedRemainningTimeSec
			trainingJobInfo.Efficiency = trainingJobInfoMongo.Efficiency
			trainingJobInfo.Speedup = trainingJobInfoMongo.Speedup
		}
		job.Info = trainingJobInfo
	}
}
