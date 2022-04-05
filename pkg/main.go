package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/service"
	"k8s.io/klog/v2"
)

func main() {
	fmt.Printf("%s (v%s)\n", config.Msg, config.Version)

	/* flags */
	kubeconfigPtr := flag.String("kubeconfig", "", "absolute path to the kubeconfig file (required if not running within pod)")
	/* flags end */

	klog.InitFlags(nil)
	if !flag.Parsed() {
		flag.Parse()
	}
	klog.InfoS(config.Msg, "version", config.Version)
	klog.InfoS("Starting service")
	klog.InfoS("Listing flags", "kubeconfig", *kubeconfigPtr)

	service := service.NewService(*kubeconfigPtr)
	err := http.ListenAndServe(":"+config.Port, service.Router)
	klog.ErrorS(err, "Service shut down")
}
