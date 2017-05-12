package trackers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDistribution(t *testing.T) {
	require := require.New(t)

	d := NewDistribution(nil, 100*time.Hour)

	values := []int32{43, 54, 56, 61, 62, 66, 68, 69, 69, 70, 71, 72, 77, 78, 79, 85, 87, 88, 89, 93, 95, 96, 98, 99, 99}
	for idx := 0; idx < len(values); idx++ {
		d.Update(values[idx], time.Duration(idx)*time.Second)
	}

	require.Equal(int32(98), d.NthPercentileTemp(90))
}
