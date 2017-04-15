package alarms

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shaseley/phonelab-go"
)

type Batch struct {
	What        string
	Start       int64
	End         int64
	Flags       int
	FlagsBinary string
}

type Alarm struct {
	What           string
	Type           int
	OrigWhen       int64
	Wakeup         bool
	Tag            string
	Flags          int
	Uid            int
	Count          int
	When           int64
	WindowLength   int64
	WhenElapsed    int64
	MaxWhenElapsed int64 `json:maxWhenElapsed`
	RepeatInterval int64
	Pid            int
	Operation      string
	CreatorPkg     string `json:omitempty`
	TargetPkg      string `json:omitempty`
	Uuid           string
	WhenRtc        int64
	MaxWhenRtc     int64
	AppPid         int `json:omitempty`
}

type SetAlarm struct {
	Func          string `json:func`
	Pid           int    `json:pid`
	Uid           int    `json:uid`
	FlagsBinary   string `json:flagsBinary`
	Flags         int    `json:flags`
	AlarmClock    string `json:alarmClock,omitempty`
	Type          int    `json:type`
	TriggerAtTime int64  `json:triggerAtTime`
	NowElapsed    int64  `json:nowELAPSED`
	Rtc           int64  `json:rtc`
	WindowLength  int64  `json:windowLength`
	Interval      int64  `json:interval`
	CreatorPkg    string `json:creatorPkg,omitempty`
	TargetPkg     string `json:targetPkg,omitempty`
}

func (sa *SetAlarm) Equals(alarm *Alarm) bool {
	if sa.Pid == alarm.Pid &&
		sa.Uid == alarm.Uid &&
		sa.Flags == alarm.Flags &&
		sa.WindowLength == alarm.WindowLength &&
		sa.Type == alarm.Type &&
		strings.Compare(sa.CreatorPkg, alarm.CreatorPkg) == 0 &&
		strings.Compare(sa.TargetPkg, alarm.TargetPkg) == 0 {
		return true
	}
	return false
}

type DeliverAlarmsLocked struct {
	Logline     *phonelab.Logline
	Alarm       `json:alarm`
	NowElapsed  int64  `json:nowELAPSED`
	Rtc         int64  `json:rtc`
	Func        string `json:func`
	WhenRtc     int64
	MaxWhenRtc  int64
	Temps       []int32
	Timestamps  []int64
	TriggerTemp int32
}

func ParseAlarm(jsonString string) (alarm *Alarm, err error) {
	alarm = new(Alarm)
	jsonString = strings.Replace(jsonString, `\"`, `"`, -1)
	err = json.Unmarshal([]byte(jsonString), alarm)
	if err != nil {
		fmt.Println(err)
		err = errors.New(fmt.Sprintf("Failed to unmarshal alarm: %v", err))
		return
	}
	return
}

func ParseDeliverAlarmsLocked(logline *phonelab.Logline) (deliverAlarm *DeliverAlarmsLocked, err error) {
	// This is an alarm trigger logline
	var data map[string]interface{}
	err = json.Unmarshal([]byte(logline.Payload.(string)), &data)
	if err != nil {
		err = errors.New(fmt.Sprintf("Failed to unmarshal: %v", err))
		return nil, err
	}
	_ = data
	deliverAlarm = new(DeliverAlarmsLocked)
	deliverAlarm.Func = data["func"].(string)
	deliverAlarm.Logline = logline
	var alarm *Alarm
	alarm, err = ParseAlarm(data["alarm"].(string))
	if err != nil {
		err = errors.New(fmt.Sprintf("Failed to unmarshal alarm: %v", err))
		deliverAlarm = nil
		return
	}
	deliverAlarm.Alarm = *alarm
	deliverAlarm.NowElapsed = int64(data["nowELAPSED"].(float64))
	deliverAlarm.Rtc = int64(data["rtc"].(float64))
	deliverAlarm.Temps = make([]int32, 0)
	deliverAlarm.Timestamps = make([]int64, 0)

	// Now fill in custom fields
	rtcFixup := int64(5 * 3600 * 1000)

	if deliverAlarm.WhenRtc == 0 {
		deliverAlarm.WhenRtc = deliverAlarm.Rtc - (deliverAlarm.NowElapsed - deliverAlarm.Alarm.WhenElapsed)
	}
	deliverAlarm.WhenRtc -= rtcFixup
	deliverAlarm.MaxWhenRtc = deliverAlarm.WhenRtc + (deliverAlarm.WindowLength)

	return
}
