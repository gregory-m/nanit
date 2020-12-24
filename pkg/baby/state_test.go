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

	assert.Equal(t, 1.0, m["temperature"], "The two words should be the same.")
	assert.Equal(t, true, m["is_night"], "The two words should be the same.")
}

func TestStateMergeSame(t *testing.T) {
	s1 := &baby.State{}
	s1.SetTemperatureMilli(10)

	s2 := &baby.State{}
	s2.SetTemperatureMilli(10)

	s3 := s1.Merge(s2)
	assert.Same(t, s1, s3)
}

func TestStateMergeDifferent(t *testing.T) {
	s1 := &baby.State{}
	s1.SetTemperatureMilli(10_000)
	s1.SetIsStreamAlive(true)

	s2 := &baby.State{}
	s2.SetTemperatureMilli(11_000)
	s2.SetHumidityMilli(20_000)
	s2.SetIsStreamAlive(true)

	s3 := s1.Merge(s2)
	assert.NotSame(t, s1, s3)
	assert.NotSame(t, s2, s3)
	assert.Equal(t, 10.0, s1.GetTemperature())

	assert.Equal(t, 11.0, s3.GetTemperature())
	assert.Equal(t, 20.0, s3.GetHumidity())
	assert.Equal(t, true, s3.GetIsStreamAlive())
}
