package algorithms

import (
	"fmt"
	"math"
	"sort"

	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	log "github.com/sirupsen/logrus"
)

type simpleLinearThreshold struct {
	BaseThreshold float64
}

func NewSimpleLinearThreshold(threshold float64) Algorithm {
	if threshold <= 0 {
		log.Errorf("Bad threshold value.")
		return nil
	}
	return &simpleLinearThreshold{threshold}
}

func (s *simpleLinearThreshold) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	totalTime := alarm.WindowLength * 1000000
	whenNanos := alarm.WhenRtc * 1000000

	totalValues := len(distribution.Temps)
	sortedKeys := make([]int, len(distribution.Temps))
	idx := 0
	for k := range distribution.Distribution {
		sortedKeys[idx] = int(k)
		idx++
	}
	sort.Ints(sortedKeys)

	nthPercentileTemp := func(percentile int) int32 {
		expectedCount := int64(math.Ceil(float64(totalValues) * (float64(percentile) / 100.0)))
		count := int64(0)
		for _, key := range sortedKeys {
			k32 := int32(key)
			count += distribution.Distribution[k32]
			if count >= expectedCount {
				return k32
			}
		}
		return int32(sortedKeys[len(sortedKeys)-1])
	}

	for idx := 0; idx < len(timestamps); idx++ {
		if timestamps[idx] < whenNanos {
			/*
				h := md5.New()
				io.WriteString(h, alarm.Logline.Line)
				checksum := fmt.Sprintf("%x", h.Sum(nil))

			*/
			//checksum := ""
			//log.Errorf("Timestamp going backwards? %v: timestamps[%d] = %v < %v", checksum, idx, timestamps[idx], whenNanos)
			_ = log.GetLevel()
			continue
		}
		percentTimeElapsed := float64((timestamps[idx]-(whenNanos))*100) / float64(totalTime)
		newThreshold := int(s.BaseThreshold + ((percentTimeElapsed * (100.0 - s.BaseThreshold)) / 100.0))
		if newThreshold > 100 {
			newThreshold = 100
		}
		nthPercentileTemp := nthPercentileTemp(newThreshold)
		//log.Debugf("percentElapsed=%d  newThreshold=%d  nthPercentileTemp=%d  temp=%d", int(percentTimeElapsed), newThreshold, nthPercentileTemp, temps[idx])
		if temps[idx] <= nthPercentileTemp {
			return temps[idx]
		}
	}
	return temps[len(temps)-1]
}

func (a *simpleLinearThreshold) Name() string {
	return fmt.Sprintf("simple-linear-threshold-%v", a.BaseThreshold)
}
