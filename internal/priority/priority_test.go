package priority

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriority_String(t *testing.T) {
	assert.Equal(t, "Low", Low.String())
	assert.Equal(t, "Unspecified", Unspecified.String())
	var p Priority
	assert.Equal(t, "", p.String())
}

func TestPriority_Ordered(t *testing.T) {
	assert.True(t, Low < Unspecified) //nolint
	var p Priority
	assert.True(t, p < Low)
}
