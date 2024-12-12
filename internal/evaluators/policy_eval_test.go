package evaluators

import (
	"fmt"
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
	d = LowPriority
	assert.Equal(t, "LowPriority", d.String())
}

func TestStringToDecision(t *testing.T) {
	tests := []struct {
		input    string
		expected Decision
		wantErr  bool
	}{
		{"Unspecified", Unspecified, false},
		{"Pending", Pending, false},
		{"Sampled", Sampled, false},
		{"NotSampled", NotSampled, false},
		{"LowPriority", LowPriority, false},
		{"InvalidInput", Unspecified, true}, // This should trigger the error case
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%s", tt.input), func(t *testing.T) {
			result, err := StringToDecision(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringToDecision(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if result != tt.expected {
				t.Errorf("StringToDecision(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
