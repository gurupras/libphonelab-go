package libphonelab

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/gurupras/go_cpuprof"
	"github.com/gurupras/go_cpuprof/post_processing"
	"github.com/gurupras/go_cpuprof/post_processing/filters"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/pbnjay/strptime"
	"github.com/stretchr/testify/require"
)

func timeStringToEpoch(str string) (time.Time, error) {
	return strptime.Parse(str, "%Y-%m-%d %H:%M:%S.%f")
}

func testHelper(require *require.Assertions, devices ...string) (*post_processing.Boot, <-chan *cpuprof.ThermalTemp) {
	device := "c75766c9053b44819717635c344535fc646e9800"
	if len(devices) > 0 {
		device = devices[0]
	}
	//args, err := shlex.Split(fmt.Sprintf("test test/ --device %v", device))
	//require.Nil(err)
	//parser := post_processing.SetupParser()
	//post_processing.ParseArgs(parser, args)

	device_files := post_processing.GetDeviceFiles("test", []string{device})

	var boot *post_processing.Boot
	for _, boots := range device_files {
		boot = boots[0]
		break
	}

	filterFunc := func(line string) bool {
		return strings.Contains(line, "thermal_temp:")
	}
	lineChannel := make(chan string, 1000)
	go boot.AsyncFilterRead(lineChannel, []filters.LineFilter{filterFunc})

	outChan := make(chan *cpuprof.ThermalTemp, 100)

	go func() {
		defer close(outChan)
		for line := range lineChannel {
			logline := cpuprof.ParseLogline(line)
			if logline == nil {
				//fmt.Fprintln(os.Stderr, "Failed to parse:", line)
				continue
			}

			// This is a thermal_temp line
			trace := cpuprof.ParseTraceFromLoglinePayload(logline)
			tt := trace.(*cpuprof.ThermalTemp)
			tt.Trace.Logline = logline
			outChan <- tt
		}
	}()
	return boot, outChan
}

func TestIsFull(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	boot, tempChan := testHelper(require)
	distribution := NewDistribution(boot, 12*time.Hour)

	for tt := range tempChan {
		temp := int32(tt.Temp)
		distribution.Update(temp, tt.Trace.Datetime)
		if distribution.IsFull() {
			diff := time.Duration(*distribution.LastTime - *distribution.StartTime)
			require.True(diff > 12*time.Hour)
			break
		}
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	boot, tempChan := testHelper(require)
	distribution := NewDistribution(boot, 48*time.Hour)

	for tt := range tempChan {
		temp := int32(tt.Temp)
		distribution.Update(temp, tt.Trace.Datetime)
		distribution.LastLogline = tt.Trace.Logline
		if distribution.IsFull() {
			break
		}
	}

	// Find the nearest temp to a known timestamp
	testTimestampStrings := []string{
		"2017-02-06 13:51:03.849685", // before
		"2017-02-06 14:38:03.849685", // before
		"2017-02-06 15:38:03.849684", // after is closer than before
		"2017-02-06 16:38:03.849686", // 0.000001 before
		"2017-02-06 20:49:13.296307", // 0.000001 before
		"2017-02-06 20:49:13.496307", // 0.000001 before
		"2017-02-06 20:49:13.496306", // exact
		"2017-02-07 15:02:35.987999", // exact last XXX: If you change the distribution length, this will fail
		"2017-02-07 15:02:35.987998", // before last XXX: If you change the distribution length, this will fail
		"2017-02-07 15:02:35.988000", // after last XXX: If you change the distribution length, this will fail
	}
	expectedTimestampStrings := []string{
		"2017-02-06 13:50:15.101710",
		"2017-02-06 13:50:15.101710",
		"2017-02-06 16:38:03.849685",
		"2017-02-06 16:38:03.849685",
		"2017-02-06 20:49:13.296306",
		"2017-02-06 20:49:13.496306",
		"2017-02-06 20:49:13.496306",
		"2017-02-07 15:02:35.987999", // XXX: If you change the distribution length, this will fail
		"2017-02-07 15:02:35.987999", // XXX: If you change the distribution length, this will fail
		"2017-02-07 15:02:35.987999", // XXX: If you change the distribution length, this will fail
	}

	testTimestamps := make([]int64, 0)
	expectedTimestamps := make([]int64, 0)

	for idx := 0; idx < len(testTimestampStrings); idx++ {
		testTimestamp, err := timeStringToEpoch(testTimestampStrings[idx])
		require.Nil(err)
		testTimestamps = append(testTimestamps, testTimestamp.UnixNano())

		expectedTimestamp, err := timeStringToEpoch(expectedTimestampStrings[idx])
		require.Nil(err)
		expectedTimestamps = append(expectedTimestamps, expectedTimestamp.UnixNano())
	}
	for idx := 0; idx < len(testTimestamps); idx++ {
		testTimestamp := testTimestamps[idx]
		expected := expectedTimestamps[idx]

		// Try linear search
		tm := time.Unix(0, testTimestamp)
		_, closestTimestamp := distribution.FindIdxByTimeLinear(tm)
		require.Equal(expected, closestTimestamp, "index=", idx)

		// Now try binary search
		tm = time.Unix(0, testTimestamp)
		_, closestTimestamp = distribution.FindIdxByTimeBinarySearch(tm)
		require.Equal(expected, closestTimestamp, "index=", idx)
	}
}

func TestSearchFakeData(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	distribution := NewDistribution(nil, 12*time.Hour)

	for idx := int32(0); idx < 1000000000; idx += rand.Int31n(1000) + 1 {
		distribution.Update(idx, idx)
	}

	for idx := rand.Int31n(100); idx < int32(len(distribution.Temps)); idx += rand.Int31n(1000) {
		t, _ := distribution.FindIdxByTimestamp(int64(idx))
		require.NotEqual(-1, t)
	}
}

func TestFailedSearch(t *testing.T) {
	// This tests for a known data point that failed
	require := require.New(t)

	boot, tempChan := testHelper(require, "84fd059afbcdff5029f8fc710580dd5d8a650346")
	distribution := NewDistribution(boot, 48*time.Hour)

	for tt := range tempChan {
		temp := int32(tt.Temp)
		distribution.Update(temp, tt.Trace.Datetime)
		distribution.LastLogline = tt.Trace.Logline
		if distribution.IsFull() {
			break
		}
	}
	knownFailedSearchStr := "2017-03-01 18:25:01.901465"
	failedTime, err := timeStringToEpoch(knownFailedSearchStr)
	require.Nil(err)

	expectedStr := "2017-03-01 18:25:01.891465"
	expectedTime, err := timeStringToEpoch(expectedStr)
	require.Nil(err)
	expectedTimestamp := expectedTime.UnixNano()

	_, closest := distribution.FindIdxByTimestampLinear(failedTime.UnixNano())
	require.Equal(expectedTimestamp, closest)

	_, closest = distribution.FindIdxByTimestampBinarySearch(failedTime.UnixNano())
	require.Equal(expectedTimestamp, closest)

	// Now try to find one near WhenRtc
	line := `84fd059afbcdff5029f8fc710580dd5d8a650346        1488392701901   1488392701901.1 e90bd6dd-175b-4abd-8fe9-a173f44f7b7f    60060605        540820.803313   2017-03-01 18:25:01.901465      884     1335    D   ThermaPlan->AlarmManagerService  {"func":"AlarmManagerService->deliverAlarmsLocked()","nowELAPSED":794808994,"rtc":1488410701916,"alarm":"{\"what\":\"ALARM\",\"type\":2,\"origWhen\":794797015,\"wakeup\":true,\"tag\":\"*walarm*:com.google.android.location.ALARM_WAKEUP_LOCATOR\",\"flags\":0,\"uid\":10014,\"count\":1,\"when\":794797015,\"windowLength\":37500,\"whenElapsed\":794797015,\"maxWhenElapsed\":794834515,\"repeatInterval\":0,\"pid\":1866,\"operation\":\"PendingIntent{95b20d6: PendingIntentRecord{69cd47c com.google.android.gms broadcastIntent}}\",\"uuid\":\"1408f6b8-a047-4f5c-9b1d-1aff7dc10988\",\"whenRtc\":1488410689938,\"maxWhenRtc\":1488410727438,\"creatorPkg\":\"com.google.android.gms\",\"targetPkg\":\"com.google.android.gms\",\"appPid\":1866}"}`
	logline := cpuprof.ParseLogline(line)
	require.NotNil(logline)
	alarm, err := alarms.ParseDeliverAlarmsLocked(logline)
	require.Nil(err)

	when := alarm.WhenRtc * 1000000
	idx, _ := distribution.FindIdxByTimestamp(when)
	require.Equal(-1, idx)

	idx, _ = distribution.FindIdxByTimestamp(when, 12*time.Second)
	require.NotEqual(-1, idx)
}
