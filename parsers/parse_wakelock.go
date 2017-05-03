package parsers

import (
	"strings"

	"github.com/shaseley/phonelab-go"
)

type ThermaPlanWakelock struct {
	Func       string `json:"func"`
	Lock       int64  `json:"lock"`
	Tag        string `json:"tag"`
	Flags      int    `json:"flags"`
	Pid        int    `json:"pid"`
	Uid        int    `json:"uid"`
	WorkSource string `json:"workSource"`
}

func (w *ThermaPlanWakelock) New() interface{} {
	return &ThermaPlanWakelock{}
}

type WakeLockType int

const (
	WAKELOCK_UNKNOWN WakeLockType = iota
	WAKELOCK_ACQUIRE WakeLockType = iota
	WAKELOCK_RELEASE WakeLockType = iota
)

func (w *ThermaPlanWakelock) Type() WakeLockType {
	if strings.Compare("acquireWakeLockInternal", w.Func) == 0 {
		return WAKELOCK_ACQUIRE
	} else if strings.Compare("releaseWakeLockInternal", w.Func) == 0 {
		return WAKELOCK_RELEASE
	} else {
		return WAKELOCK_UNKNOWN
	}
}

func NewThermaPlanWakelockParser() phonelab.Parser {
	return phonelab.NewJSONParser(&ThermaPlanWakelock{})
}
