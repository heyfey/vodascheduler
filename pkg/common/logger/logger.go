package logger

import (
	"flag"
	"path"
	"time"

	"k8s.io/klog/v2"
)

// Constants for logging
const (
	Name = "Voda Scheduler"
	User = "heyfey"

	// TODO: replace these
	LogDir  = "/logs"
	LogName = "voda"

	LogToStderr     = "false"
	AlsoLogtoStderr = "true"
	V               = "4"
)

// TODO: Considering switch to klog
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md#replacing-fatal-calls

// Usage:
// log := logger.NewLogger().WithName("xxx").WithValues("xxx", xxx)
// defer logger.Flush()
// ...do some logging

// InitLogger initializes logger with constants for logging
func InitLogger() {
	logName := LogName + "-" + time.Now().Format("20060102-030405") + ".log"
	logPath := path.Join(LogDir, logName)

	klog.InitFlags(nil)
	flag.Set("v", V)
	flag.Set("log_file", logPath)
	flag.Set("logtostderr", LogToStderr)
	flag.Set("alsologtostderr", AlsoLogtoStderr)
	flag.Parse()
}
