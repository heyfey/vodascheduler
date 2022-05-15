package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/service/service"
	"k8s.io/klog/v2"
)

func main() {
	fmt.Printf("%s (v%s) - Training Service\n", config.Msg, config.Version)

	klog.InitFlags(nil)
	if !flag.Parsed() {
		flag.Parse()
	}
	klog.InfoS(config.Msg, "version", config.Version)
	klog.InfoS("Starting training service")

	service := service.NewService()
	err := http.ListenAndServe(":"+config.Port, service.Router)
	klog.ErrorS(err, "Service shut down")
}
