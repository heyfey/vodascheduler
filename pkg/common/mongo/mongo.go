package mongo

import (
	"os"

	"github.com/heyfey/vodascheduler/pkg/common/logger"
	"gopkg.in/mgo.v2"
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

// ConnectMongo connects to a mongo session.
// It returns a pointer to the session, or an error if the connection attempt fails.
// TODO: May require username and password in the future
func ConnectMongo() *mgo.Session {
	log := logger.GetLogger()
	logger.Flush()

	host := os.Getenv("MONGODB_SVC_SERVICE_HOST")
	port := os.Getenv("MONGODB_SVC_SERVICE_PORT")

	mongoURI := host + ":" + port
	session, err := mgo.Dial(mongoURI)
	if err != nil {
		log.Error(err, "Could not connect to mongodb", "mongoURI", mongoURI)
		panic(err)
	}
	return session
}
