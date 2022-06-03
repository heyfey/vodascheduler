package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/scheduler"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	fmt.Printf("%s (v%s) - Scheduler\n", config.Msg, config.Version)

	/* flags */
	gpuType := flag.String("gpu", "default", "GPU type of this scheduler")
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file (required if not running within pod)")
	/* flags end */

	klog.InitFlags(nil)
	if !flag.Parsed() {
		flag.Parse()
	}
	klog.InfoS(config.Msg, "version", config.Version)
	klog.InfoS("Starting scheduler")
	klog.InfoS("Listing flags", "gpu", *gpuType, "kubeconfig", *kubeconfig)

	var kConfig *rest.Config
	var err error
	if *kubeconfig != "" {
		kConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		kConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.ErrorS(err, "Failed to build config")
		klog.Flush()
		os.Exit(1)
	}

	sched, err := scheduler.NewScheduler(*gpuType, kConfig)
	if err != nil {
		klog.ErrorS(err, "Failed to create scheduler", "gpu", *gpuType)
		klog.Flush()
		os.Exit(1)
	}

	err = http.ListenAndServe(":"+"55588", sched.Router) // TODO(heyfey): assign port
	klog.ErrorS(err, "Scheduler shut down")
}
