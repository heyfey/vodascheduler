package service

import (
	"github.com/gorilla/mux"
	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/rabbitmq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/amqp"
	"gopkg.in/mgo.v2"
)

type Service struct {
	Router  *mux.Router
	session *mgo.Session
	mqConn  *amqp.Connection
	Metrics ServiceMetrics
}

func NewService() *Service {
	s := &Service{
		Router:  mux.NewRouter(),
		session: mongo.ConnectMongo(),
		mqConn:  rabbitmq.ConnectRabbitMQ(),
	}
	s.initRoutes()
	s.initServiceMetrics()
	return s
}

func (s *Service) initRoutes() {
	s.Router.HandleFunc("/", homePage)
	s.Router.HandleFunc(config.EntryPoint, s.createTrainingJobHandler()).Methods("POST")
	s.Router.HandleFunc(config.EntryPoint, s.deleteTrainingJobHandler()).Methods("DELETE")
	s.Router.Handle("/metrics", promhttp.Handler())
}
