package algorithms

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	log "github.com/sirupsen/logrus"
)

type SimpleLinearThreshold struct {
}

func (s *SimpleLinearThreshold) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	totalTime := alarm.WindowLength * 1000000
	whenNanos := alarm.WhenRtc * 1000000

	for idx := 0; idx < len(timestamps); idx++ {
		if timestamps[idx] < whenNanos {
			/*
				h := md5.New()
				io.WriteString(h, alarm.Logline.Line)
				checksum := fmt.Sprintf("%x", h.Sum(nil))

				log.Errorf("Timestamp going backwards? %v: timestamps[%d] = %v < %v", checksum, idx, timestamps[idx], whenNanos)
			*/
			_ = log.GetLevel()
			continue
		}
		percentTimeElapsed := float64((timestamps[idx]-(whenNanos))*100) / float64(totalTime)
		newThreshold := int(25 + ((percentTimeElapsed * 75.0) / 100.0))
		if newThreshold > 100 {
			newThreshold = 100
		}
		if temps[idx] <= distribution.NthPercentileTemp(newThreshold) {
			return temps[idx]
		}
	}
	return temps[len(temps)-1]
}

func (a *SimpleLinearThreshold) Name() string {
	return "simple-linear-threshold"
}
