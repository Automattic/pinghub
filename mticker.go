package main

import (
	"sync"
	"time"
)

type mTicker struct {
	mux         sync.Mutex
	subscribers subscribers
	ticker      *time.Ticker
	stopCh      chan struct{}
	dropped     int
}

type subscribers map[*subscriber]interface {
}

type subscriber struct {
	tick chan time.Time
}

// creates and starts a new ticker that can
// have subscribed channels to receive ticks
func newMTicker(interval time.Duration) *mTicker {
	t := &mTicker{
		subscribers: make(subscribers),
		stopCh:      make(chan struct{}, 1),
		ticker:      time.NewTicker(interval),
	}

	go t.tick()
	return t
}

func newSubscriber() *subscriber {
	return &subscriber{
		tick: make(chan time.Time, 1),
	}
}

// Subscribe returns a channel to which ticks will be delivered. Ticks that
// can't be delivered to the channel, because it is not ready to receive, are
// discarded.
func (t *mTicker) subscribe() *subscriber {
	t.mux.Lock()
	defer t.mux.Unlock()
	sub := newSubscriber()
	t.subscribers[sub] = nil
	return sub
}

// closes subscriber channel and
// removes from subscribers map
func (t *mTicker) unsubscribe(subscriber *subscriber) {
	t.mux.Lock()
	defer t.mux.Unlock()
	close(subscriber.tick)
	delete(t.subscribers, subscriber)
}

// Stop stops the ticker, and closes
// all subscribed channels
func (t *mTicker) stop() {
	if t.stopCh != nil {
		t.ticker.Stop()
		t.stopCh <- struct{}{}
		// close all subscribed time chans
		for sub := range t.subscribers {
			t.unsubscribe(sub)
		}
	}
}

// broadcast ticks to all
// subscribed channels
func (t *mTicker) tick() {
	for {
		select {
		case tick := <-t.ticker.C:
			t.mux.Lock()
			for sub := range t.subscribers {
				select {
				case sub.tick <- tick:
				default:
					t.dropped++
				}
			}
			t.mux.Unlock()
		case <-t.stopCh:
			return
		}
	}
}
