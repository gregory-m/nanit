package main

import (
	reflect "reflect"
	"sync"
)

// BabyState Struct holding information about state of a single baby
type BabyState struct {
	Temperature *int32
	Humidity    *int32
}

// Merge Merges non-nil values of an argument to the state.
// Returns ptr to new state if changes
// Returns ptr to old state if not changed
func (state *BabyState) Merge(stateUpdate *BabyState) *BabyState {
	newState := &BabyState{}
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

// StateManager State manager context
type StateManager struct {
	BabiesByUID      map[string]BabyState
	Subscribers      map[*chan bool]func(babyUID string, state BabyState)
	StateMutex       sync.RWMutex
	SubscribersMutex sync.RWMutex
}

// NewStateManager State manager constructor
func NewStateManager() *StateManager {
	return &StateManager{
		BabiesByUID: make(map[string]BabyState),
		Subscribers: make(map[*chan bool]func(babyUID string, state BabyState)),
	}
}

// Update updates baby info in thread safe manner
func (manager *StateManager) Update(babyUID string, stateUpdate BabyState) {
	var newState *BabyState

	manager.StateMutex.Lock()
	defer manager.StateMutex.Unlock()

	if babyState, ok := manager.BabiesByUID[babyUID]; ok {
		newState = babyState.Merge(&stateUpdate)
		if newState == &babyState {
			return
		}
	} else {
		newState = (&BabyState{}).Merge(&stateUpdate)
	}

	manager.BabiesByUID[babyUID] = *newState
	go manager.notifySubscribers(babyUID, *newState)
}

// Subscribe Registers function to be called on every update
// Returns unsubscribe function
func (manager *StateManager) Subscribe(callback func(babyUID string, state BabyState)) func() {
	unsubscribeC := make(chan bool, 1)

	manager.SubscribersMutex.Lock()
	manager.Subscribers[&unsubscribeC] = callback
	manager.SubscribersMutex.Unlock()

	manager.StateMutex.RLock()
	for babyUID, babyState := range manager.BabiesByUID {
		go callback(babyUID, babyState)
	}

	manager.StateMutex.RUnlock()

	return func() {
		manager.SubscribersMutex.Lock()
		delete(manager.Subscribers, &unsubscribeC)
		manager.SubscribersMutex.Unlock()
	}
}

func (manager *StateManager) notifySubscribers(babyUID string, state BabyState) {
	manager.SubscribersMutex.RLock()

	for _, callback := range manager.Subscribers {
		go callback(babyUID, state)
	}

	manager.SubscribersMutex.RUnlock()
}
