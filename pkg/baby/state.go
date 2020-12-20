package baby

import (
	reflect "reflect"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// State - struct holding information about state of a single baby
type State struct {
	LocalStreamingInitiated *bool
	IsNight                 *bool
	TemperatureMilli        *int32
	HumidityMilli           *int32
}

// Merge - Merges non-nil values of an argument to the state.
// Returns ptr to new state if changes
// Returns ptr to old state if not changed
func (state *State) Merge(stateUpdate *State) *State {
	newState := &State{}
	changed := false

	currReflect := reflect.ValueOf(state).Elem()
	newReflect := reflect.ValueOf(newState).Elem()
	patchReflect := reflect.ValueOf(stateUpdate).Elem()

	for i := 0; i < currReflect.NumField(); i++ {
		currField := currReflect.Field(i)
		newField := newReflect.Field(i)
		patchField := patchReflect.Field(i)

		patchFieldValue := reflect.Value(patchField)
		if patchFieldValue.IsNil() {
			// FIXME: copy values not just pointers
			newField.Set(reflect.Value(currField))
		} else {
			changed = true
			// FIXME: copy values not just pointers
			newField.Set(patchFieldValue)
		}
	}

	if changed {
		return newState
	}

	return state
}

var upperCaseRX = regexp.MustCompile("[A-Z]+")

// AsMap - returns K/V map of non-nil properties
func (state *State) AsMap() map[string]interface{} {
	m := make(map[string]interface{})

	r := reflect.ValueOf(state).Elem()
	t := r.Type()
	for i := 0; i < r.NumField(); i++ {
		f := r.Field(i)
		if !f.IsNil() && f.Type().Kind() == reflect.Ptr {
			name := t.Field(i).Name
			var value interface{}

			if f.Type().Elem().Kind() == reflect.Int32 {
				value = f.Elem().Int()

				if strings.HasSuffix(name, "Milli") {
					name = strings.TrimSuffix(name, "Milli")
					value = float32(value.(int64)) / 1000
				}
			} else {
				value = f.Elem().Interface()
			}

			name = strings.ToLower(name[0:1]) + name[1:]
			name = upperCaseRX.ReplaceAllStringFunc(name, func(m string) string {
				return "_" + strings.ToLower(m)
			})

			m[name] = value
		}
	}

	return m
}

// EnhanceLogEvent - appends non-nil properties to a log event
func (state *State) EnhanceLogEvent(e *zerolog.Event) *zerolog.Event {
	for key, value := range state.AsMap() {
		e.Interface(key, value)
	}

	return e
}

// SetTemperatureMilli - mutates field, returns itself
func (state *State) SetTemperatureMilli(value int32) *State {
	state.TemperatureMilli = &value
	return state
}

// GetTemperature - returns temperature as floating point
func (state *State) GetTemperature() float32 {
	if state.TemperatureMilli != nil {
		return float32(*state.TemperatureMilli) / 1000
	}

	return 0
}

// SetHumidityMilli - mutates field, returns itself
func (state *State) SetHumidityMilli(value int32) *State {
	state.HumidityMilli = &value
	return state
}

// GetHumidity - returns humidity as floating point
func (state *State) GetHumidity() float32 {
	if state.HumidityMilli != nil {
		return float32(*state.HumidityMilli) / 1000
	}

	return 0
}

// SetLocalStreamingInitiated - mutates field, returns itself
func (state *State) SetLocalStreamingInitiated(value bool) *State {
	state.LocalStreamingInitiated = &value
	return state
}

// GetLocalStreamingInitiated - safely returns value
func (state *State) GetLocalStreamingInitiated() bool {
	if state.LocalStreamingInitiated != nil {
		return *state.LocalStreamingInitiated
	}

	return false
}

// SetIsNight - mutates field, returns itself
func (state *State) SetIsNight(value bool) *State {
	state.IsNight = &value
	return state
}
