package parsers

import (
	"testing"

	"github.com/shaseley/phonelab-go"
	"github.com/stretchr/testify/require"
)

func TestParseRelease(t *testing.T) {
	require := require.New(t)

	parser := phonelab.NewLogcatParser()
	tparser := NewThermaPlanWakelockParser()

	line := `1ead348f9487bd344881d4ef684c44429fafd237        1493078400126   1493078400126.0 9586e22e-ac54-42b6-a6bd-edf422a2ff3d    1708881 12994.150191    2017-04-25 00:00:00.126999      962     1993    D       ThermaPlan->WakeLock    {"func":"releaseWakeLockInternal","lock":180860690,"tag":"GmsChimeraWakelockManager","flags":0,"pid":962}`

	logline, err := parser.Parse(line)
	require.Nil(err)

	obj, err := tparser.Parse(logline.Payload.(string))
	wkObj := obj.(*ThermaPlanWakelock)

	require.Equal("releaseWakeLockInternal", wkObj.Func)
	require.Equal(int64(180860690), wkObj.Lock)
	require.Equal("GmsChimeraWakelockManager", wkObj.Tag)
	require.Equal(0, wkObj.Flags)
	require.Equal(962, wkObj.Pid)
}

func TestParseAcquire(t *testing.T) {
	require := require.New(t)

	parser := phonelab.NewLogcatParser()
	tparser := NewThermaPlanWakelockParser()

	line := `1ead348f9487bd344881d4ef684c44429fafd237        1493078400169   1493078400169.0 9586e22e-ac54-42b6-a6bd-edf422a2ff3d    1708873 12994.045908    2017-04-25 00:00:00.169990      962     2393    D       ThermaPlan->WakeLock    {"func":"acquireWakeLockInternal","lock":173172381,"flags":1,"tag":"GmsChimeraWakelockManager","workSource":"WorkSource{10014 com.google.android.gms}","uid":10014,"pid":3827}`

	logline, err := parser.Parse(line)
	require.Nil(err)

	obj, err := tparser.Parse(logline.Payload.(string))
	wkObj := obj.(*ThermaPlanWakelock)

	require.Equal("acquireWakeLockInternal", wkObj.Func)
	require.Equal(int64(173172381), wkObj.Lock)
	require.Equal(1, wkObj.Flags)
	require.Equal("GmsChimeraWakelockManager", wkObj.Tag)
	require.Equal("WorkSource{10014 com.google.android.gms}", wkObj.WorkSource)
	require.Equal(10014, wkObj.Uid)
	require.Equal(3827, wkObj.Pid)
}
