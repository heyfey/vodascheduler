package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	go func() {
		err := http.ListenAndServe(":"+config.Port, service.Router)
		if err != nil {
			klog.ErrorS(err, "error starting API service")
		}
	}() // run in background
	klog.InfoS("API up and running")

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	signal := <-signalChannel
	klog.InfoS("Received termination, gracefully shutdown", signal)

	tc, _ := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	service.Shutdown(tc)
}
