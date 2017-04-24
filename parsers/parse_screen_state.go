package parsers

import (
	"regexp"
	"strconv"

	"github.com/shaseley/phonelab-go"
)

type ScreenStateParser struct {
	regex       *regexp.Regexp
	subexpNames []string
}

func NewScreenStateParser() phonelab.Parser {
	regex := regexp.MustCompile(`Set power mode=(?P<mode>\d), type=(?P<type>\d) flinger=(?P<addr>0x[a-z0-9]+)`)
	names := regex.SubexpNames()
	return &ScreenStateParser{regex, names}
}

type ScreenState struct {
	Mode int
	Type int
	Addr int64
}

func (s *ScreenStateParser) Parse(payload string) (interface{}, error) {
	matches := s.regex.FindStringSubmatch(payload)
	if len(matches) == 0 {
		return nil, nil
	}

	mode, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}
	t, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}
	addr, err := strconv.ParseInt(matches[3][2:], 16, 64)
	if err != nil {
		return nil, err
	}

	return &ScreenState{
		Mode: mode,
		Type: t,
		Addr: addr,
	}, nil
}
