package logger

import (
	"flag"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

// Constants for logging
const (
	Name            = "Celeste"
	User            = "heyfey"
	LogPath         = "/home/heyfey/celeste/celeste/logs/my_file.log" // TODO: replace this
	LogToStderr     = "false"
	AlsoLogtoStderr = "true"
	V               = "5"
)

// TODO: Considering switch to klog
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md#replacing-fatal-calls

// Usage:
// log := logger.NewLogger().WithName("xxx").WithValues("xxx", xxx)
// defer logger.Flush()
// ...do some logging

// InitLogger initializes logger with constants for logging
func InitLogger() {
	klog.InitFlags(nil)
	flag.Set("v", V)
	flag.Set("log_file", LogPath)
	flag.Set("logtostderr", LogToStderr)
	flag.Set("alsologtostderr", AlsoLogtoStderr)
	flag.Parse()
}

// GetLogger constructs a new logger
func GetLogger() logr.Logger {
	logger := klogr.New()
	return logger
}

// Flush simply calls klog.flush()
// Remember to call this before return from functions do logging
// to ensure output to file correctly.
func Flush() {
	klog.Flush()
}
