package baby

import (
	"sync"
)

// StateManager - state manager context
type StateManager struct {
	BabiesByUID      map[string]State
	Subscribers      map[*chan bool]func(babyUID string, state State)
	StateMutex       sync.RWMutex
	SubscribersMutex sync.RWMutex
}

// NewStateManager - state manager constructor
func NewStateManager() *StateManager {
	return &StateManager{
		BabiesByUID: make(map[string]State),
		Subscribers: make(map[*chan bool]func(babyUID string, state State)),
	}
}

// Update - updates baby info in thread safe manner
func (manager *StateManager) Update(babyUID string, stateUpdate State) {
	var newState *State

	manager.StateMutex.Lock()
	defer manager.StateMutex.Unlock()

	if babyState, ok := manager.BabiesByUID[babyUID]; ok {
		newState = babyState.Merge(&stateUpdate)
		if newState == &babyState {
			return
		}
	} else {
		newState = (&State{}).Merge(&stateUpdate)
	}

	manager.BabiesByUID[babyUID] = *newState
	go manager.notifySubscribers(babyUID, *newState)
}

// Subscribe - registers function to be called on every update
// Returns unsubscribe function
func (manager *StateManager) Subscribe(callback func(babyUID string, state State)) func() {
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

func (manager *StateManager) notifySubscribers(babyUID string, state State) {
	manager.SubscribersMutex.RLock()

	for _, callback := range manager.Subscribers {
		go callback(babyUID, state)
	}

	manager.SubscribersMutex.RUnlock()
}
