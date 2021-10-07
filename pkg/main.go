package main

import (
	"fmt"
	"net/http"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/common/logger"
	"github.com/heyfey/vodascheduler/pkg/service"
)

func main() {
	fmt.Printf("%s (v%s)\n", config.Msg, config.Version)

	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info(config.Msg, "version", config.Version)
	log.Info("Starting service")

	service := service.NewService()
	err := http.ListenAndServe(":"+config.Port, service.Router)
	log.Error(err, "Service shut down")
}
