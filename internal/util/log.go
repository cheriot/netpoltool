package util

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var Log = Init()

func Init() *logrus.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		PadLevelText:     true,
	})
	log.SetLevel(logrus.ErrorLevel)
	return log
}

func SetLogLevel(logLevel string) error {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("unable to parse log level %s: %w", logLevel, err)
	}
	Log.SetLevel(level)
	return nil
}
