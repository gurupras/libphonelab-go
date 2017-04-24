package trackers

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/gurupras/gocommons"
	"github.com/shaseley/phonelab-go"
	"github.com/stretchr/testify/assert"
)

func testFgBgTracker(fgbgTracker *FgBgTracker, assert *assert.Assertions) int64 {
	filePath := filepath.Join("inputs", "fgbg.gz")

	fstruct, err := gocommons.Open(filePath, os.O_RDONLY, gocommons.GZ_TRUE)
	assert.Nil(err, "Failed to open test input file:", err)
	defer fstruct.Close()
	fstruct.Seek(0, 0)

	reader, err := fstruct.Reader(0)
	assert.Nil(err, "Failed to get reader to file:", err)

	reader.Split(bufio.ScanLines)

	filters := fgbgTracker.Filter.AsLineFilterArray()

	count := 0
	for reader.Scan() {
		line := reader.Text()
		logline := phonelab.ParseLogline(line)
		//if fgbgFilter.CurrentState&Background != 0 {
		//fmt.Println(line)
		//}
		valid := true
		for _, f := range filters {
			valid = valid && f(logline)
			if !valid {
				break
			}
		}
		if !valid {
			continue
		}
		_ = line
		count++
	}
	return int64(count)
}

func testFgBgState(state FgBgState, filter *Filter, assert *assert.Assertions) int64 {
	fgbgTracker := NewFgBgTracker(filter)
	fgbgTracker.TrackerState = state
	return testFgBgTracker(fgbgTracker, assert)

}

func TestFgBgTracker(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	filter := New()

	// Only show unknown lines
	filter = New()
	assert.Equal(int64(6), testFgBgState(FgBgUnknown, filter, assert), "Got unexpected number of lines for unknown fgbg-state")

	// Only show bg lines
	filter = New()
	assert.Equal(int64(3), testFgBgState(Background, filter, assert), "Got unexpected number of lines for background fgbg-state")

	// Only show fg lines
	filter = New()
	assert.Equal(int64(7), testFgBgState(Foreground, filter, assert), "Got unexpected number of lines for background fgbg-state")
}
