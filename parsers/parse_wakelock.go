package parsers

import "github.com/shaseley/phonelab-go"

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

func NewThermaPlanWakelockParser() phonelab.Parser {
	return phonelab.NewJSONParser(&ThermaPlanWakelock{})
}
