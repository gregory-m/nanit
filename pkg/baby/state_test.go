package baby_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
)

func TestStateAsMap(t *testing.T) {
	s := baby.State{}
	s.SetTemperatureMilli(1000)
	s.SetIsNight(true)

	m := s.AsMap()

	assert.Equal(t, float32(1.0), m["temperature"], "The two words should be the same.")
	assert.Equal(t, true, m["is_night"], "The two words should be the same.")
}
