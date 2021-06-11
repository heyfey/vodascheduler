// https://dev.to/lucasnevespereira/write-a-rest-api-in-golang-following-best-practices-pe9

package service

import (
	"github.com/gorilla/mux"
	"github.com/heyfey/celeste/pkg/jobmaster"
)

type Service struct {
	JM     *jobmaster.JobMaster
	Router *mux.Router
}

func NewService() *Service {
	s := &Service{
		JM:     jobmaster.NewJobMaster(),
		Router: mux.NewRouter(),
	}
	s.initRoutes()
	return s
}

func (s *Service) initRoutes() {
	s.Router.HandleFunc("/", homePage)
	s.Router.HandleFunc("/training", s.createTrainingJobHandler()).Methods("POST")
	s.Router.HandleFunc("/training", s.deleteTrainingJobHandler()).Methods("DELETE")
	s.Router.HandleFunc("/training", s.getAllTrainingJobHandler()).Methods("GET")
}
