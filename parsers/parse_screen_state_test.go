package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScreenState(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	line := `Set power mode=2, type=0 flinger=0xb2ae4000`

	parser := NewScreenStateParser()
	obj, err := parser.Parse(line)
	require.Nil(err)
	require.NotNil(obj)

	ss, ok := obj.(*ScreenState)
	require.True(ok)

	assert.Equal(2, ss.Mode)
	assert.Equal(0, ss.Type)
	assert.Equal(int64(0xb2ae4000), ss.Addr)
}
