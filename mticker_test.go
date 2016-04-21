package main

import (
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	ticker := newMTicker(2 * time.Second)

	// assert no subscribers
	if len(ticker.subscribers) != 0 {
		t.Fatal("Expectation: 0, Received:", len(ticker.subscribers))
	}

	ticker.subscribe()
	if len(ticker.subscribers) != 1 {
		t.Fatal("Expectation: 1, Received:", len(ticker.subscribers))
	}
}

func TestUnsubscribe(t *testing.T) {
	ticker := newMTicker(2 * time.Second)
	sub := ticker.subscribe()

	// assert 1 subscribers
	if len(ticker.subscribers) != 1 {
		t.Fatal("Expectation: 1, Received:", len(ticker.subscribers))
	}

	// assert chan unsubscribed
	ticker.unsubscribe(sub)
	if len(ticker.subscribers) != 0 {
		t.Fatal("Expectation: 0, Received:", len(ticker.subscribers))
	}

	// assert chan closed
	_, ok := <-sub.tick
	if ok {
		t.Fatal("Expectation: tick channel should be closed, Received: open channel")
	}
}

func TestTick(t *testing.T) {
	ticker := newMTicker(2 * time.Second)
	sub1 := ticker.subscribe()
	sub2 := ticker.subscribe()
	sub3 := ticker.subscribe()

	// tick is already called indirectly
	// through "newMTicker()"
	time.Sleep(2)

	// assert time stamps are passed
	// to subscribing channels
	t1, ok1 := <-sub1.tick
	t2, ok2 := <-sub2.tick
	t3, ok3 := <-sub3.tick

	if !ok1 || !ok2 || !ok3 || !(t1 == t2 && t1 == t3) {
		t.Fatal("Expecation: all subscribed channels receive identical time stamps, Received:", t1, t2, t3)
	}
}

func TestStop(t *testing.T) {
	ticker := newMTicker(2 * time.Second)
	sub1 := ticker.subscribe()
	sub2 := ticker.subscribe()

	ticker.stop()

	// assert all subscribing
	// channels closed
	// assert chan closed
	_, ok1 := <-sub1.tick
	_, ok2 := <-sub2.tick

	if ok1 || ok2 {
		t.Fatal("Expectation: all tick channels should be closed, Received: open channel")
	}

}
