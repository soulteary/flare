package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var instance = logrus.New()

func init() {
	instance.Formatter = new(logrus.TextFormatter)
	instance.Formatter.(*logrus.TextFormatter).DisableColors = false
	instance.Formatter.(*logrus.TextFormatter).DisableTimestamp = false
	instance.Formatter.(*logrus.TextFormatter).FullTimestamp = true

	// TODO Automatically adjust log output level based on environment startup configuration
	instance.Level = logrus.TraceLevel
	instance.Out = os.Stdout
}

func GetLogger() *logrus.Logger {
	return instance
}
