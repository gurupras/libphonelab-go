package alarms

import (
	"encoding/json"
	"testing"

	"github.com/shaseley/phonelab-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	parser := phonelab.NewLogcatParser()

	line := `cb63cb9bb9ad1ea9fcfab53403820c7c084621ab        1483145199410   1483145199410.5 f99bb66e-3282-42ca-b109-f79cc0739e93    456949  180.281579      2016-12-31 00:46:39.410999      963     1363    D       ThermaPlan->AlarmManagerService      {"func":"AlarmManagerService->deliverAlarmsLocked()","nowELAPSED":180277,"rtc":1483163199424,"alarm":"{\"what\":\"ALARM\",\"type\":2,\"origWhen\":137253,\"wakeup\":true,\"tag\":\"*walarm*:com.whatsapp.messaging.h.CLIENT_PINGER_ACTION\",\"flags\":0,\"uid\":10093,\"count\":1,\"when\":137253,\"windowLength\":180000,\"whenElapsed\":137253,\"maxWhenElapsed\":317253,\"repeatInterval\":240000,\"pid\":2923,\"operation\":\"PendingIntent{54e032d: PendingIntentRecord{8fd4062 com.whatsapp broadcastIntent}}\",\"uuid\":\"d0f2e5e4-c36f-47de-9e56-86db0504c053\",\"whenRtc\":1483163156401,\"maxWhenRtc\":1483163336401,\"creatorPkg\":\"com.whatsapp\",\"targetPkg\":\"com.whatsapp\",\"appPid\":10402}"}`

	logline, err := parser.Parse(line)
	require.Nil(err)
	require.NotNil(logline, "Failed to parse logline")

	dal, err := ParseDeliverAlarmsLocked(logline)
	require.Nil(err, "Failed to parse:", err)
	require.NotNil(dal, "Failed to parse DeliverAlarmsLocked", err)

	assert.Equal("AlarmManagerService->deliverAlarmsLocked()", dal.Func)
	assert.Equal(int64(180277), dal.NowElapsed)
	assert.Equal(int64(1483163199424), dal.Rtc)

	data := make(map[string]interface{})
	err = json.Unmarshal([]byte(logline.Payload.(string)), &data)
	assert.Nil(err, "Failed to convert string to JSON")
	alarmString := data["alarm"].(string)
	alarm, err := ParseAlarm(alarmString)
	assert.NotNil(alarm, "Failed to parse alarm:", err)
	assert.Equal(dal.Alarm, alarm, "Alarms not equal")

	// Now check each field of alarm
	assert.Equal("ALARM", alarm.What)
	assert.Equal(2, alarm.Type)
	assert.Equal(int64(137253), alarm.OrigWhen)
	assert.Equal(true, alarm.Wakeup)
	assert.Equal("*walarm*:com.whatsapp.messaging.h.CLIENT_PINGER_ACTION", alarm.Tag)
	assert.Equal(0, alarm.Flags)
	assert.Equal(10093, alarm.Uid)
	assert.Equal(1, alarm.Count)
	assert.Equal(int64(137253), alarm.When)
	assert.Equal(int64(180000), alarm.WindowLength)
	assert.Equal(int64(137253), alarm.WhenElapsed)
	assert.Equal(int64(317253), alarm.MaxWhenElapsed)
	assert.Equal(int64(240000), alarm.RepeatInterval)
	assert.Equal(2923, alarm.Pid)
	assert.Equal("PendingIntent{54e032d: PendingIntentRecord{8fd4062 com.whatsapp broadcastIntent}}", alarm.Operation)
	assert.Equal("d0f2e5e4-c36f-47de-9e56-86db0504c053", alarm.Uuid)
	assert.Equal(int64(1483163156401), alarm.WhenRtc)
	assert.Equal(int64(1483163336401), alarm.MaxWhenRtc)
	assert.Equal("com.whatsapp", alarm.CreatorPkg)
	assert.Equal("com.whatsapp", alarm.TargetPkg)
	assert.Equal(10402, alarm.AppPid)
}

func TestParser(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	parser := NewDeliverAlarmsLockedParser()

	line := `{"func":"AlarmManagerService->deliverAlarmsLocked()","nowELAPSED":180277,"rtc":1483163199424,"alarm":"{\"what\":\"ALARM\",\"type\":2,\"origWhen\":137253,\"wakeup\":true,\"tag\":\"*walarm*:com.whatsapp.messaging.h.CLIENT_PINGER_ACTION\",\"flags\":0,\"uid\":10093,\"count\":1,\"when\":137253,\"windowLength\":180000,\"whenElapsed\":137253,\"maxWhenElapsed\":317253,\"repeatInterval\":240000,\"pid\":2923,\"operation\":\"PendingIntent{54e032d: PendingIntentRecord{8fd4062 com.whatsapp broadcastIntent}}\",\"uuid\":\"d0f2e5e4-c36f-47de-9e56-86db0504c053\",\"whenRtc\":1483163156401,\"maxWhenRtc\":1483163336401,\"creatorPkg\":\"com.whatsapp\",\"targetPkg\":\"com.whatsapp\",\"appPid\":10402}"}`

	obj, err := parser.Parse(line)
	require.Nil(err, "Failed to parse:", err)
	require.NotNil(obj, "Failed to parse DeliverAlarmsLocked", err)

	dal, ok := obj.(*DeliverAlarmsLocked)
	require.NotNil(dal)
	require.True(ok)

	assert.Equal("AlarmManagerService->deliverAlarmsLocked()", dal.Func)
	assert.Equal(int64(180277), dal.NowElapsed)
	assert.Equal(int64(1483163199424), dal.Rtc)

	data := make(map[string]interface{})
	err = json.Unmarshal([]byte(line), &data)
	assert.Nil(err, "Failed to convert string to JSON")
	alarmString := data["alarm"].(string)
	alarm, err := ParseAlarm(alarmString)
	assert.NotNil(alarm, "Failed to parse alarm:", err)
	assert.Equal(dal.Alarm, alarm, "Alarms not equal")

	// Now check each field of alarm
	assert.Equal("ALARM", alarm.What)
	assert.Equal(2, alarm.Type)
	assert.Equal(int64(137253), alarm.OrigWhen)
	assert.Equal(true, alarm.Wakeup)
	assert.Equal("*walarm*:com.whatsapp.messaging.h.CLIENT_PINGER_ACTION", alarm.Tag)
	assert.Equal(0, alarm.Flags)
	assert.Equal(10093, alarm.Uid)
	assert.Equal(1, alarm.Count)
	assert.Equal(int64(137253), alarm.When)
	assert.Equal(int64(180000), alarm.WindowLength)
	assert.Equal(int64(137253), alarm.WhenElapsed)
	assert.Equal(int64(317253), alarm.MaxWhenElapsed)
	assert.Equal(int64(240000), alarm.RepeatInterval)
	assert.Equal(2923, alarm.Pid)
	assert.Equal("PendingIntent{54e032d: PendingIntentRecord{8fd4062 com.whatsapp broadcastIntent}}", alarm.Operation)
	assert.Equal("d0f2e5e4-c36f-47de-9e56-86db0504c053", alarm.Uuid)
	assert.Equal(int64(1483163156401), alarm.WhenRtc)
	assert.Equal(int64(1483163336401), alarm.MaxWhenRtc)
	assert.Equal("com.whatsapp", alarm.CreatorPkg)
	assert.Equal("com.whatsapp", alarm.TargetPkg)
	assert.Equal(10402, alarm.AppPid)

}
