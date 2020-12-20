package baby

import (
	reflect "reflect"

	"github.com/rs/zerolog"
)

// State - struct holding information about state of a single baby
type State struct {
	LocalStreamingInitiated *bool
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

// EnhanceLogEvent - appends non-nil properties to a log event
func (state *State) EnhanceLogEvent(e *zerolog.Event) *zerolog.Event {
	r := reflect.ValueOf(state).Elem()
	t := r.Type()
	for i := 0; i < r.NumField(); i++ {
		f := r.Field(i)
		if !f.IsNil() {

			e.Interface(t.Field(i).Name, f.Interface())
		}
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
