package baby

import (
	reflect "reflect"
)

// State - struct holding information about state of a single baby
type State struct {
	Temperature *int32
	Humidity    *int32
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
