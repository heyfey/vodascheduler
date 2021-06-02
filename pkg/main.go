package main

import (
	"fmt"
	"time"

	"github.com/heyfey/celeste/pkg/jobmaster"
)

func main() {
	jm := jobmaster.NewJobMaster()

	// wait for job master to start
	time.Sleep(time.Duration(5) * time.Second)

	jm.CreateTrainingJob("../examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml")
	for {
		time.Sleep(time.Duration(5) * time.Second)
		fmt.Println("tick")
	}
}
