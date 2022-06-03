package mongo

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"gopkg.in/mgo.v2"
	"k8s.io/klog/v2"
)

const (
	maxNumGpu = 32
)

type TrainingJobInfo struct {
	Name                       string             `bson:"name" json:"name"`
	GpuTimeSec                 float32            `bson:"gpu_time_sec" json:"gpu_time_sec"`
	CurrentEpoch               int32              `bson:"current_epoch" json:"current_epoch"`
	Efficiency                 map[string]float32 `bson:"efficiency" json:"efficiency"`
	ElaspedTimeSec             float32            `bson:"elasped_time_sec" json:"elasped_time_sec"`
	EpochTimeSec               map[string]float32 `bson:"epoch_time_sec" json:"epoch_time_sec"`
	EstimatedRemainningTimeSec float32            `bson:"estimated_remainning_time_sec" json:"estimated_remainning_time_sec"`
	RemainningEpochs           int32              `bson:"remainning_epochs" json:"remainning_epochs"`
	RunningTimeSec             float32            `bson:"running_time_sec" json:"running_time_sec"`
	Speedup                    map[string]float32 `bson:"speedup" json:"speedup"`
	StepTimeSec                map[string]float32 `bson:"step_time_sec" json:"step_time_sec"`
	TotalEpochs                int32              `bson:"total_epochs" json:"total_epochs"`
}

type JobRunning struct {
	Name string `bson:"name" json:"name"`
}

// ConnectMongo connects to a mongo session. It returns a pointer to the session,
// TODO(heyfey): or an error if the connection attempt fails.
// TODO(heyfey): May require username and password in the future
func ConnectMongo() *mgo.Session {

	// Find service IP and port from kube-dns (CoreDNS)
	// my-svc.my-namespace.svc.cluster-domain.example
	host := "mongodb-svc.voda-scheduler.svc.cluster.local"
	iprecords, err := net.LookupIP("mongodb-svc.voda-scheduler.svc.cluster.local")
	if err != nil {
		klog.ErrorS(err, "Failed to look up mongodb service host IP", "host", host)
		klog.Flush()
		os.Exit(1)
	}
	ip := iprecords[0]

	// _my-port-name._my-port-protocol.my-svc.my-namespace.svc.cluster-domain.example
	portName := "voda-mongodb"
	protocal := "tcp"
	domainName := "mongodb-svc.voda-scheduler.svc.cluster.local"
	_, addrs, err := net.LookupSRV(portName, protocal, domainName)
	if err != nil {
		klog.ErrorS(err, "Failed to look up mongodb service port", "portName", portName, "protocal", protocal,
			"domainName", domainName)
		klog.Flush()
		os.Exit(1)
	}
	port := addrs[0].Port

	url := ip.String() + ":" + fmt.Sprint(port)
	session, err := mgo.Dial(url)
	if err != nil {
		klog.ErrorS(err, "Failed to connect to mongodb", "url", url)
		klog.Flush()
		os.Exit(1)
	} else {
		klog.InfoS("Connected to mongodb", "url", url)
	}
	return session
}

// CreateBaseJobInfo creates a TrainingJobInfo that assumes linear speedup.
func CreateBaseJobInfo(jobName string) TrainingJobInfo {
	speedup := map[string]float32{"0": 0.0}
	efficiency := map[string]float32{"0": 0.0}
	time := map[string]float32{"0": 0.0}
	for i := 1; i <= maxNumGpu+1; i++ {
		speedup[strconv.Itoa(i)] = float32(i)
		efficiency[strconv.Itoa(i)] = float32(1)
		time[strconv.Itoa(i)] = float32(1)
	}

	info := TrainingJobInfo{
		Name:                       jobName,
		GpuTimeSec:                 0.0,
		CurrentEpoch:               0,
		Efficiency:                 efficiency,
		ElaspedTimeSec:             0.0,
		EpochTimeSec:               time,
		EstimatedRemainningTimeSec: 0.0,
		RemainningEpochs:           1,
		RunningTimeSec:             0.0,
		Speedup:                    speedup,
		StepTimeSec:                time,
		TotalEpochs:                1,
	}

	return info
}
