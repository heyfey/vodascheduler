package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/common/logger"
	"github.com/heyfey/vodascheduler/pkg/service"
)

func main() {
	fmt.Printf("%s (v%s)\n", config.Msg, config.Version)

	// flag definition should placed before logger.InitLogger()
	/* flags */
	kubeconfigPtr := flag.String("kubeconfig", "", "absolute path to the kubeconfig file (required if not running within pod)")
	/* flags end */

	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info(config.Msg, "version", config.Version)
	log.Info("Starting service")

	if !flag.Parsed() {
		flag.Parse()
	}
	log.Info("Listing flags", "kubeconfig", *kubeconfigPtr)

	service := service.NewService(*kubeconfigPtr)
	err := http.ListenAndServe(":"+config.Port, service.Router)
	log.Error(err, "Service shut down")
}
