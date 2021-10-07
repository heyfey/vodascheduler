// https://dev.to/lucasnevespereira/write-a-rest-api-in-golang-following-best-practices-pe9

package service

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: homePage")
	fmt.Fprintf(w, "Voda Scheduler - DLT jobs scheduler")
}

func (s *Service) createTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: createTrainingJob")

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, err.Error())
			return
		}

		name, err := s.JM.CreateTrainingJob(reqBody)
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Training job created: %s\nView your logs by:\n%s", name, "    kubectl logs "+name+"-launcher")
		}
	}
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
		err = s.JM.DeleteTrainingJob(job)
		if err != nil {
			fmt.Fprintf(w, err.Error())
		} else {
			fmt.Fprintf(w, "Training job deleted: %s", job)
		}
	}
}

func (s *Service) getAllTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: getAllTrainingJob")
		fmt.Fprintf(w, s.JM.GetAllTrainingJob())
	}
}
