package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/rabbitmq"
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	v1 "k8s.io/api/core/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

const (
	databaseNameJobInfo       = "job_info"
	databaseNameJobMetadata   = "job_metadata"
	collectionNameJobMetadata = "v1beta1"
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: homePage")
	fmt.Fprintf(w, "Voda Scheduler - DLT jobs scheduler - Training Service")
}

func (s *Service) createTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: createTrainingJob")
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, err.Error())
			return
		}

		name, err := s.CreateTrainingJob(reqBody)
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Training job created: %s\nView your logs by:\n%s", name, "    kubectl logs "+name+"-launcher")
		}
	}
}

// CreateTrainingJob
//   1. Validate job spec
//   2. Get or create base job info by job category
//   3. Pre-process job spec
//   4. Create training job
//     4.1. Insert job info to db
//     4.2. Insert job metadata to db
//   5. Publish msg to mq, which is meant to be subscribed by the scheduler
func (s *Service) CreateTrainingJob(data []byte) (string, error) {
	mpijob, err := bytesToMPIJob(data)
	if err != nil {
		klog.InfoS("Failed to convert data to mpijob", "err", err)
		return "", err
	}
	// 1. TODO(heyfey): Validate job spec

	jobName := mpijob.GetName()
	jobCategory := jobName

	// 2. Get or create base job info by job category
	info, err := s.getOrCreateBaseJobInfo(mpijob)
	if err != nil {
		klog.InfoS("Failed to create training job", "err", err, "job", jobName)
		return "", err
	}

	// 3. Pre-process job spec
	// extend training job name with timestamp
	now := time.Now()
	jobName = jobName + "-" + now.Format("20060102-030405")
	mpijob.SetName(jobName)
	setEnvJobName(mpijob, jobName)

	// 4. Create training job
	t, err := trainingjob.NewTrainingJob(mpijob, jobCategory, now)
	if err != nil {
		klog.InfoS("Failed to create training job", "err", err, "job", jobName)
		return "", err
	}

	sess := s.session.Clone()
	defer sess.Close()

	// 4.1 Insert job info to db
	info = initJobInfo(info, jobName, t.Config.Epochs)
	cJobInfo := sess.DB(databaseNameJobInfo).C(jobCategory)
	err = cJobInfo.Insert(info)
	if err != nil {
		klog.ErrorS(err, "Failed to insert record to mongo", "database", databaseNameJobInfo,
			"collection", jobCategory, "job", jobName)
		return "", err
	}

	// 4.2. Insert job metadata to db
	cJobMetadata := sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata)
	err = cJobMetadata.Insert(t)
	if err != nil {
		klog.InfoS("Failed to create training job", "err", err, "job", jobName)
		return "", err
	}

	// 5. Publish msg to mq, the msg is meant to be subscribed by the scheduler
	msg := rabbitmq.Msg{Verb: rabbitmq.VerbCreate, JobName: jobName}
	err = rabbitmq.PublishToQueue(s.mqConn, t.GpuType, msg)
	if err != nil {
		// Remove the just inserted record if the publishing failed. This is to
		// avoid inconsistency between the DB and the message queue.
		err2 := cJobInfo.Remove(bson.M{"name": jobName})
		if err2 != nil {
			klog.ErrorS(err2, "Failed to remove record", "job", jobName,
				"database", databaseNameJobInfo, "collection", jobCategory)
		}
		err2 = cJobMetadata.Remove(bson.M{"job_name": jobName})
		if err2 != nil {
			klog.ErrorS(err2, "Failed to remove record", "job", jobName)
		}
		return "", err
	}

	klog.InfoS("Created training job", "job", jobName)
	return jobName, nil
}

func bytesToMPIJob(data []byte) (*kubeflowv1.MPIJob, error) {
	var err error

	if data, err = yaml2.ToJSON(data); err != nil {
		return nil, err
	}

	// TODO(heyfey): convert to mpijob, tfjob or pytorchjob
	mpijob := &kubeflowv1.MPIJob{}
	if err = json.Unmarshal(data, mpijob); err != nil {
		return nil, err
	}

	return mpijob, nil
}

// setEnvJobName sets the environment variable "JOB_NAME" of mpijob
func setEnvJobName(mpijob *kubeflowv1.MPIJob, name string) {
	launcherSpec := mpijob.Spec.MPIReplicaSpecs["Launcher"]
	// there should be only one container in the spec
	env := launcherSpec.Template.Spec.Containers[0].Env
	for i := 0; i < len(env); i++ {
		if env[i].Name == string(types.JobName) {
			env[i].Value = name
			return
		}
	}
	// environment variable "JOB_NAME" not found, add it by ourselves
	env = append(env, v1.EnvVar{Name: string(types.JobName), Value: name})
}

// getOrCreateBaseJobInfo finds record (history information) of the job by its
// category. If not found, create and a basic info and insert to database.
//  DB layout:
//  db.collection.record
//  <databaseNameJobInfo>.<job_category>.<job_name>
// We simply identify job category by its metadata.name, which means jobs with same
// metadata.name are considered the same and assumed to have similar characteristics.
func (s *Service) getOrCreateBaseJobInfo(mpijob *kubeflowv1.MPIJob) (mongo.TrainingJobInfo, error) {
	sess := s.session.Clone()
	defer sess.Close()

	jobName := mpijob.GetName()
	jobCategory := jobName

	info := mongo.TrainingJobInfo{}
	err := sess.DB(databaseNameJobInfo).C(jobCategory).Find(bson.M{"name": jobName}).One(&info)
	if err != nil {
		if err == mgo.ErrNotFound {
			klog.InfoS("Could not find job info, making basic info", "err", err,
				"database", databaseNameJobInfo, "category", jobCategory, "job", jobName)

			info = mongo.CreateBaseJobInfo(jobName)
			err = sess.DB(databaseNameJobInfo).C(jobCategory).Insert(info)
			if err != nil {
				klog.InfoS("Failed to insert job info", "err", err,
					"database", databaseNameJobInfo, "category", jobCategory, "job", jobName)
				return info, err
			}
		} else {
			return info, err
		}
	}
	return info, nil
}

// initJobInfo creates job info base on given base job info. Also estimates
// remainning training time.
// TODO(heyfey): Currently remainning time estimation is epoch-based, for some
// jobs we may need to do estimation base on steps instead of epochs.
func initJobInfo(basicInfo mongo.TrainingJobInfo, jobName string, epochs int) mongo.TrainingJobInfo {
	info := basicInfo
	info.Name = jobName
	info.CurrentEpoch = 0
	info.ElaspedTimeSec = 0.0
	info.EstimatedRemainningTimeSec = float32(epochs) * basicInfo.EpochTimeSec["1"]
	info.GpuTimeSec = 0.0
	info.RemainningEpochs = int32(epochs)
	info.RunningTimeSec = 0.0
	info.TotalEpochs = int32(epochs)
	return info
}

func (s *Service) deleteTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: deleteTrainingJob")

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, err.Error())
			return
		}

		var job string
		err = json.Unmarshal(reqBody, &job)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(err)
			return
		}

		err = s.DeleteTrainingJob(job)
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Training job deleted: %s", job)
		}
	}
}

// DeleteTrainingJob
// 1. Delete job metadata from db
// 2. Publish msg to mq, which is meant to be subscribed by the scheduler
func (s *Service) DeleteTrainingJob(jobName string) error {
	// 1. Delete metadata from db
	sess := s.session.Clone()
	defer sess.Close()

	c := sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata)

	// Make sure the training job exist and get job metadata
	t := trainingjob.TrainingJob{}
	err := c.Find(bson.M{"job_name": jobName}).One(&t)
	if err != nil {
		if err == mgo.ErrNotFound {
			klog.InfoS("Attempted to delete a non-existing training job", "err", err, "job", jobName)
		} else {
			klog.InfoS("Failed to delete training job", "err", err, "job", jobName)
		}
		return err
	}

	err = c.Remove(bson.M{"job_name": jobName})
	if err != nil {
		klog.InfoS("Failed to delete training job", "err", err, "job", jobName)
		return err
	}

	// 2. Publish deletion msg to mq
	// TODO(heyfey): Do we need to publish msg if delete a completed job?
	msg := rabbitmq.Msg{Verb: rabbitmq.VerbDelete, JobName: jobName}
	err = rabbitmq.PublishToQueue(s.mqConn, t.GpuType, msg)
	if err != nil {
		klog.InfoS("Failed to delete training job", "err", err, "job", jobName)
		return err
	}

	klog.InfoS("Deleted training job", "job", jobName)
	return nil
}
