package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/allocator/allocator"
	"k8s.io/klog/v2"
)

func main() {
	fmt.Printf("%s (v%s) - Resource Allocator\n", config.Msg, config.Version)

	klog.InitFlags(nil)
	if !flag.Parsed() {
		flag.Parse()
	}
	klog.InfoS(config.Msg, "version", config.Version)
	klog.InfoS("Starting resource allocator")

	ra := allocator.NewResourceAllocator()
	err := http.ListenAndServe(":"+"55588", ra.Router)
	klog.ErrorS(err, "Resource allocator shut down")
}
