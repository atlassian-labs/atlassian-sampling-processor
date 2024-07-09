package evaluators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecisionStringer(t *testing.T) {
	d := Unspecified
	assert.Equal(t, "Unspecified", d.String())
	d = Pending
	assert.Equal(t, "Pending", d.String())
	d = Sampled
	assert.Equal(t, "Sampled", d.String())
	d = NotSampled
	assert.Equal(t, "NotSampled", d.String())
}
