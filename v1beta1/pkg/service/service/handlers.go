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
// 1. Validate job spec
// 2. Pre-process job spec
// 3. Insert job metadata to db
// 4. Publish msg to mq, which is meant to be subscribed by the scheduler
func (s *Service) CreateTrainingJob(data []byte) (string, error) {
	mpijob, err := bytesToMPIJob(data)
	if err != nil {
		klog.InfoS("Failed to convert data to mpijob", "err", err)
		return "", err
	}
	// 1. TODO(heyfey): Validate job spec

	// 2. Pre-process job spec
	//
	// In Voda, we identify a job by its metadata.name, which means jobs with
	// same metadata.name are considered the same and assumed to have similar characteristics.
	// We also extend all jobs' metadata.name with a timestamp to distinguish them.
	//
	// First find record (history information) of the job by its metadata.name in mongodb.
	// If not present, create and a basic info and insert it to the database.
	// Then, insert a new record with modified job name (with timestamp added)
	//
	// db structure:
	// | database             | collection | records                        |
	// | -------------------- | ---------- | ------------------------------ |
	// | databaseNameJobInfo  | job name 1 | job name with unique timestamp |
	// | databaseNameJobInfo  | job name 2 | job name with unique timestamp |
	//
	jobName := mpijob.GetName()
	jobCollection := jobName

	sess := s.session.Clone()
	defer sess.Close()

	info := mongo.TrainingJobInfo{}
	err = sess.DB(databaseNameJobInfo).C(jobCollection).Find(bson.M{"name": jobName}).One(&info)
	if err != nil {
		if err == mgo.ErrNotFound {
			klog.InfoS("Could not find job info in mongo", "err", err, "database", databaseNameJobInfo,
				"collection", jobCollection, "jobName", jobName)

			info = mongo.CreateBaseJobInfo(jobName)
			err = sess.DB(databaseNameJobInfo).C(jobCollection).Insert(info)
			if err != nil {
				klog.InfoS("Failed to insert record in mongo", "err", err, "database", databaseNameJobInfo,
					"collection", jobCollection, "jobName", jobName)
				return "", err
			}
		} else {
			return "", err
		}
	}

	// add timestamp to training job name
	now := time.Now()
	// jobName = jobName + "-" + now.Format("20060102-030405")
	mpijob.SetName(jobName)
	setEnvJobName(mpijob, jobName)

	// 3. Insert meta to db
	t, err := trainingjob.NewTrainingJob(*mpijob, jobCollection, now)
	if err != nil {
		klog.InfoS("Failed to create training job", "err", err, "jobName", jobName)
		return "", err
	}
	err = sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata).Insert(t)
	if err != nil {
		klog.InfoS("Failed to create training job", "err", err, "jobName", jobName)
		return "", err
	}

	// 4. Publish msg to mq
	msg := rabbitmq.Msg{Verb: rabbitmq.VerbCreate, JobName: jobName}
	err = rabbitmq.PublishToQueue(s.mqConn, t.GpuType, msg)
	if err != nil {
		return "", err
	}

	klog.InfoS("Created training job", "jobName", jobName)
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

func (s *Service) deleteTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: deleteTrainingJob")

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, err.Error())
			return
		}

		job := string(reqBody)
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
			klog.InfoS("Attempted to delete a non-existing training job", "err", err, "jobName", jobName)
		} else {
			klog.InfoS("Failed to delete training job", "err", err, "jobName", jobName)
		}
		return err
	}

	err = c.Remove(bson.M{"job_name": jobName})
	if err != nil {
		klog.InfoS("Failed to delete training job", "err", err, "jobName", jobName)
		return err
	}

	// 2. Publish deletion msg to mq
	// TODO(heyfey): Do we need to publish msg if delete a completed job?
	msg := rabbitmq.Msg{Verb: rabbitmq.VerbDelete, JobName: jobName}
	err = rabbitmq.PublishToQueue(s.mqConn, t.GpuType, msg)
	if err != nil {
		klog.InfoS("Failed to delete training job", "err", err, "jobName", jobName)
		return err
	}

	klog.InfoS("Deleted training job", "job", jobName)
	return nil
}
