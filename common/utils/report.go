package utils

import (
	"sentioxyz/sentio-core/common/log"
	"time"

	"golang.org/x/exp/slices"
)

func PrintTimeReport(title string, used []time.Duration) {
	slices.Sort(used)
	var sum time.Duration
	for _, u := range used {
		sum += u
	}
	log.Info("==================")
	log.Info(title, ":")
	log.Info("min: ", used[0])
	log.Info("50%: ", used[len(used)/2])
	log.Info("90%: ", used[len(used)*9/10])
	log.Info("99%: ", used[len(used)*99/100])
	log.Info("max: ", used[len(used)-1])
	log.Info("avg: ", sum/time.Duration(len(used)))
	log.Info("total: ", sum)
	log.Info("count: ", len(used))
	log.Info("------------------")
}
