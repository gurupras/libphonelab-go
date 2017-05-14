package algorithms

import (
	"os"
	"testing"
	"time"

	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	log "github.com/sirupsen/logrus"
)

func TestSLT(t *testing.T) {
	d := trackers.NewDistribution(nil, 100*time.Second)
	for idx := 0; idx < 100; idx++ {
		d.Update(int32(idx), int64(time.Duration(idx)*time.Second))
	}

	alg := &SimpleLinearThreshold{}

	alarm := &alarms.DeliverAlarmsLocked{}
	alarm.Alarm = &alarms.Alarm{}
	alarm.WindowLength = 10000
	alarm.WhenRtc = 60000

	temp := alg.Process(alarm, 35, d.Temps[60:], d.Timestamps[60:], d)
	log.Infof("slt-temp: %v", temp)
}

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)
	code := m.Run()
	os.Exit(code)
}
