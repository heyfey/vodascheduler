package allocator

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/heyfey/vodascheduler/pkg/algorithm"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/mgo.v2"
	"k8s.io/klog/v2"
)

const entryPoint = "/allocation"

type ResourceAllocator struct {
	session *mgo.Session
	Router  *mux.Router
}

func NewResourceAllocator() *ResourceAllocator {
	ra := &ResourceAllocator{
		session: mongo.ConnectMongo(),
		Router:  mux.NewRouter(),
	}
	ra.initRoutes()
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

		allocation, err := ra.allocateResource(newReq)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err)
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(allocation)
		}
	}
}

func (ra *ResourceAllocator) allocateResource(req AllocationRequest) (types.JobScheduleResult, error) {
	klog.InfoS("Allocating resource",
		"algorithm", req.AlgorithmName, "numGpus", req.NumGpu, "scheduler", req.SchedulerID)
	defer klog.V(4).InfoS("Finished allocating resource",
		"algorithm", req.AlgorithmName, "numGpus", req.NumGpu, "scheduler", req.SchedulerID)

	// 1. get all job info from DB

	// 2. allocate resources via algorithm
	algorithm, err := algorithm.NewAlgorithmFactory(req.AlgorithmName, req.SchedulerID)
	if err != nil {
		klog.ErrorS(err, "Failed to create algorithm",
			"algorithm", req.AlgorithmName, "scheduler", req.SchedulerID)
		return nil, err
	}

	allocation := algorithm.Schedule(req.ReadyJobs, req.NumGpu)
	return allocation, nil
}
