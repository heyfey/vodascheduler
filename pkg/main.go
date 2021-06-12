package main

import (
	"fmt"
	"net/http"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/service"
)

const (
	port    = "10000"
	version = "0.0.1"
	msg     = "Celeste - DLT jobs scheduler"
)

func main() {
	fmt.Printf("%s (v%s)\n", msg, version)

	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info(msg, "version", version)
	log.Info("Starting service")

	service := service.NewService()
	err := http.ListenAndServe(":"+port, service.Router)
	log.Error(err, "Service shut down")
}
