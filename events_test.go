package main

import "testing"

func TestEventHubDisconnectsSubscriberWhenItsBufferIsFull(t *testing.T) {
	hub := newEventHub()
	events, ok := hub.subscribe("bin", true)
	if !ok {
		t.Fatal("subscribe = false, want true")
	}

	first := SummarizedRequest{Id: "1"}
	hub.publish("bin", first)
	hub.publish("bin", SummarizedRequest{Id: "2"})

	if got := <-events; got != first {
		t.Errorf("buffered event = %#v, want %#v", got, first)
	}
	if _, ok := <-events; ok {
		t.Error("subscriber remains open after its buffer filled")
	}
}

func TestEventHubClosesBinSubscribersAndRejectsNewSubscribersAfterShutdown(t *testing.T) {
	hub := newEventHub()
	first, ok := hub.subscribe("first", true)
	if !ok {
		t.Fatal("first subscribe = false, want true")
	}
	second, ok := hub.subscribe("second", true)
	if !ok {
		t.Fatal("second subscribe = false, want true")
	}

	hub.closeBin("first")
	if _, ok := <-first; ok {
		t.Error("first bin subscriber remains open after bin close")
	}
	if events, ok := hub.subscribe("first", true); ok || events != nil {
		t.Errorf("subscribe to deleted bin = (%v, %t), want (nil, false)", events, ok)
	}
	select {
	case <-second:
		t.Error("closing one bin closed another bin's subscriber")
	default:
	}
	hub.openBin("first")
	if events, ok := hub.subscribe("first", true); !ok || events == nil {
		t.Errorf("subscribe after reopening bin = (%v, %t), want (channel, true)", events, ok)
	}

	hub.close()
	if _, ok := <-second; ok {
		t.Error("subscriber remains open after hub close")
	}
	if events, ok := hub.subscribe("third", true); ok || events != nil {
		t.Errorf("subscribe after close = (%v, %t), want (nil, false)", events, ok)
	}
}

func TestEventHubClosesGuestStreamsAndKeepsTheOwnersWhenSharingIsWithdrawn(t *testing.T) {
	hub := newEventHub()
	owner, ok := hub.subscribe("bin", true)
	if !ok {
		t.Fatal("owner subscribe = false, want true")
	}
	guest, ok := hub.subscribe("bin", false)
	if !ok {
		t.Fatal("guest subscribe = false, want true")
	}

	hub.closeGuestStreams("bin")

	if _, ok := <-guest; ok {
		t.Error("guest stream remains open after sharing was withdrawn")
	}
	select {
	case <-owner:
		t.Error("owner stream closed when sharing was withdrawn")
	default:
	}

	// The owner is still a subscriber, not merely an unclosed channel.
	summary := SummarizedRequest{Id: "1"}
	hub.publish("bin", summary)
	if got := <-owner; got != summary {
		t.Errorf("owner event = %#v, want %#v", got, summary)
	}
}
