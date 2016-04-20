package main

import (
	"sync"
	"time"
)

type mTicker struct {
	mux         sync.Mutex // Protects chans slice
	subscribers subscribers

	tickerMux sync.Mutex // Used to sync start/stop
	ticker    *time.Ticker
	stopCh    chan struct{}
	stopped   bool
	dropped   int
}

type subscribers map[*subscriber]interface {
}

type subscriber struct {
	tick chan time.Time
}

// creates and starts a new ticker
// that can have subscribed channels to receive
// ticks
func newMTicker(interval time.Duration) *mTicker {
	t := &mTicker{
		subscribers: make(subscribers),
	}

	go func() {
		t.tickerMux.Lock()
		stopped := t.stopped

		if !stopped {
			t.stopCh = make(chan struct{}, 1)
			t.ticker = time.NewTicker(interval)
		}
		t.tickerMux.Unlock()

		if !stopped {
			t.tick()
		}
	}()
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

func (t *mTicker) unsubscribe(subscriber *subscriber) {
	t.mux.Lock()
	defer t.mux.Unlock()

	close(subscriber.tick)
	delete(t.subscribers, subscriber)
}

// Stop stops the ticker, and closes
// all subscribed channels
func (t *mTicker) stop() {
	t.tickerMux.Lock()
	defer t.tickerMux.Unlock()

	if !t.stopped && t.stopCh != nil {
		// close all subscribed time chans
		for sub := range t.subscribers {
			close(sub.tick)
		}
		t.ticker.Stop()
		t.stopCh <- struct{}{}
	}
	t.stopped = true
}

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
