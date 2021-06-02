// https://github.com/IBM/FfDL/blob/master/trainer/trainer/queue.go

package scheduler

import (
	"errors"

	"github.com/heyfey/celeste/pkg/common/trainingjob"
)

// JobQueue represents the functionality of a queue
type JobQueue interface {
	Enqueue(trainingjob.TrainingJob)
	Delete(string) error
	Get(string) (*trainingjob.TrainingJob, error)
	Size() int
	Empty() bool
	// Lock()
	// Unlock()
}

// TrainingJobQueue is a JobQueue
// TODO: Considering store the whole queue in mongo
type TrainingJobQueue struct {
	Queue []trainingjob.TrainingJob
	// queueID string // unique identifier for this queue so we can acquire lock
	// mtx     sync.RWMutex
	// session         *mgo.Session
	// database        string
	// queueCollection string
	// lockCollection  string
}

// newTrainingJobQueue creates a new queue for training jobs
func newTrainingJobQueue() (*TrainingJobQueue, error) {
	// sid := "test"
	q := &TrainingJobQueue{
		Queue: make([]trainingjob.TrainingJob, 0),
		// queueID: fmt.Sprintf("queue-%s", sid),
	}
	return q, nil
}

// Enqueue adds a training job to the queue
// jobmaster should acquire the lock before calling Enqueue()
func (q *TrainingJobQueue) Enqueue(t trainingjob.TrainingJob) {
	q.Queue = append(q.Queue, t)
}

// Delete removes a training job from any position in the queue while
// keeping the original order
// jobmaster should acquire the lock before calling Delete()
func (q *TrainingJobQueue) Delete(id string) error {
	found, index := q.index(id)
	if found != true {
		return errors.New("training job not found")
	} else {
		q.Queue = append(q.Queue[:index], q.Queue[index+1:]...)
		return nil
	}
}

// Get finds a training job int the queue and return it
func (q *TrainingJobQueue) Get(id string) (*trainingjob.TrainingJob, error) {
	found, index := q.index(id)
	if found != true {
		return nil, errors.New("training job not found")
	} else {
		return &q.Queue[index], nil
	}
}

// find finds the index of a training job id in the queue
func (q *TrainingJobQueue) index(id string) (bool, int) {
	for i, t := range q.Queue {
		if t.JobName == id {
			return true, i
		}
	}
	return false, 0
}

// Size returns the number of elements in the queue
func (q *TrainingJobQueue) Size() int {
	size := len(q.Queue)
	return size
}

// Empty returns whether the queue has any jobs
func (q *TrainingJobQueue) Empty() bool {
	size := q.Size()
	return size == 0
}

// Lock acquires a distributed lock in mongo
// trainer should use this when pulling jobs, so that multiple trainers do not peek/submit the same job to lcm
// func (q *TrainingJobQueue) Lock() {
// 	q.mtx.Lock()
// }

// Unlock releases the lock in mongo
// func (q *TrainingJobQueue) Unlock() {
// 	q.mtx.Unlock()
// }
