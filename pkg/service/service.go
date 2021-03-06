// https://dev.to/lucasnevespereira/write-a-rest-api-in-golang-following-best-practices-pe9

package service

import (
	"github.com/gorilla/mux"
	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/jobmaster"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	JM     *jobmaster.JobMaster
	Router *mux.Router
}

func NewService(kubeConfig string) *Service {
	s := &Service{
		JM:     jobmaster.NewJobMaster(kubeConfig),
		Router: mux.NewRouter(),
	}
	s.initRoutes()
	return s
}

func (s *Service) initRoutes() {
	s.Router.HandleFunc("/", homePage)
	s.Router.HandleFunc(config.EntryPoint, s.createTrainingJobHandler()).Methods("POST")
	s.Router.HandleFunc(config.EntryPoint, s.deleteTrainingJobHandler()).Methods("DELETE")
	s.Router.HandleFunc(config.EntryPoint, s.getAllTrainingJobHandler()).Methods("GET")
	s.Router.Handle("/metrics", promhttp.Handler())
}
