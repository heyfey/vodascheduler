package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	// "github.com/heyfey/vodascheduler/pkg/common/mongo"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	// session := mongo.ConnectMongo()
	// fmt.Println(session)

	content, err := ioutil.ReadFile("tensorflow2-keras-mnist-elastic.yaml")
	check(err)
	fmt.Printf("File contents: \n%s", content)

	if content, err = yaml2.ToJSON(content); err != nil {
		panic(err)
	}
	fmt.Println(content)

	mpijob := &kubeflowv1.MPIJob{}
	if err = json.Unmarshal(content, mpijob); err != nil {
		panic(err)
	}
	fmt.Println(mpijob)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func bytesToMPIJob(data []byte) (*kubeflowv1.MPIJob, error) {
	var err error

	if data, err = yaml2.ToJSON(data); err != nil {
		return nil, err
	}

	mpijob := &kubeflowv1.MPIJob{}
	if err = json.Unmarshal(data, mpijob); err != nil {
		return nil, err
	}

	return mpijob, nil
}
