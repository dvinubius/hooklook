package main

import "testing"

func TestEventHubDisconnectsSubscriberWhenItsBufferIsFull(t *testing.T) {
	hub := newEventHub()
	events, ok := hub.subscribe("bin")
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
	first, ok := hub.subscribe("first")
	if !ok {
		t.Fatal("first subscribe = false, want true")
	}
	second, ok := hub.subscribe("second")
	if !ok {
		t.Fatal("second subscribe = false, want true")
	}

	hub.closeBin("first")
	if _, ok := <-first; ok {
		t.Error("first bin subscriber remains open after bin close")
	}
	if events, ok := hub.subscribe("first"); ok || events != nil {
		t.Errorf("subscribe to deleted bin = (%v, %t), want (nil, false)", events, ok)
	}
	select {
	case <-second:
		t.Error("closing one bin closed another bin's subscriber")
	default:
	}
	hub.openBin("first")
	if events, ok := hub.subscribe("first"); !ok || events == nil {
		t.Errorf("subscribe after reopening bin = (%v, %t), want (channel, true)", events, ok)
	}

	hub.close()
	if _, ok := <-second; ok {
		t.Error("subscriber remains open after hub close")
	}
	if events, ok := hub.subscribe("third"); ok || events != nil {
		t.Errorf("subscribe after close = (%v, %t), want (nil, false)", events, ok)
	}
}
