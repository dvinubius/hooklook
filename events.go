package main

import "sync"

// EventHub keeps ephemeral request-event subscribers grouped by bin. It is
// deliberately independent of the database: reconnecting clients refetch the
// persisted request list rather than expecting missed events to be replayed.
type EventHub struct {
	mu          sync.Mutex
	subscribers map[string]map[chan SummarizedRequest]viewer
	closedBins  map[string]struct{}
	closed      bool
}

// viewer records the one thing a stream's fate depends on beyond its bin:
// whose stream it is. Turning sharing off revokes the guests' reading, not
// the owner's, so the two are told apart here rather than at publish time.
type viewer struct {
	owner bool
}

func newEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[string]map[chan SummarizedRequest]viewer),
		closedBins:  make(map[string]struct{}),
	}
}

var eventHub = newEventHub()

// subscribe gives each client room for exactly one event. A false result means
// the server is shutting down or the bin was deleted before the stream opened.
func (hub *EventHub) subscribe(binCode string, owner bool) (chan SummarizedRequest, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return nil, false
	}
	if _, closed := hub.closedBins[binCode]; closed {
		return nil, false
	}

	events := make(chan SummarizedRequest, 1)
	if hub.subscribers[binCode] == nil {
		hub.subscribers[binCode] = make(map[chan SummarizedRequest]viewer)
	}
	hub.subscribers[binCode][events] = viewer{owner: owner}
	return events, true
}

// openBin permits subscriptions for a newly persisted bin. It also makes a
// future code reuse safe after a previously deleted bin was marked closed.
func (hub *EventHub) openBin(binCode string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.closedBins, binCode)
}

func (hub *EventHub) unsubscribe(binCode string, events chan SummarizedRequest) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.removeSubscriber(binCode, events)
}

// publish never waits for a network client. A subscriber that has not consumed
// its one buffered event is disconnected; its browser reconnects and refetches
// the persisted list, which is the documented recovery path.
func (hub *EventHub) publish(binCode string, summary SummarizedRequest) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for events := range hub.subscribers[binCode] {
		select {
		case events <- summary:
		default:
			hub.removeSubscriber(binCode, events)
		}
	}
}

// closeGuestStreams ends the streams guests hold on a bin and leaves the
// owner's alone. Turning sharing off takes the bin back from its guests; the
// owner is still reading it, and their page should not have to reconnect.
func (hub *EventHub) closeGuestStreams(binCode string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for events, who := range hub.subscribers[binCode] {
		if !who.owner {
			hub.removeSubscriber(binCode, events)
		}
	}
}

// closeBin ends every stream for a deleted bin. It is safe to call after the
// database deletion has succeeded.
func (hub *EventHub) closeBin(binCode string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.closeBinLocked(binCode)
}

// close releases all streams during process shutdown. It is idempotent.
func (hub *EventHub) close() {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return
	}
	hub.closed = true
	for binCode := range hub.subscribers {
		hub.closeBinLocked(binCode)
	}
}

func (hub *EventHub) removeSubscriber(binCode string, events chan SummarizedRequest) {
	subscribers := hub.subscribers[binCode]
	if _, ok := subscribers[events]; !ok {
		return
	}
	delete(subscribers, events)
	close(events)
	if len(subscribers) == 0 {
		delete(hub.subscribers, binCode)
	}
}

func (hub *EventHub) closeBinLocked(binCode string) {
	for eventChannels := range hub.subscribers[binCode] {
		close(eventChannels)
	}
	delete(hub.subscribers, binCode)
	hub.closedBins[binCode] = struct{}{}
}
