package main

import (
	"flag"
	gometrics "github.com/rcrowley/go-metrics"
	"io"
	"os"
	"time"
)

type metrics struct {
	log  io.Writer
	reg  gometrics.Registry
	tick time.Duration
}

var m *metrics

func init() {
	m = &metrics{
		log: os.Stderr,
		reg: gometrics.DefaultRegistry,
		tick: time.Duration(60) * time.Second,
	}
	flag.DurationVar(&m.tick, "metrics.tick", m.tick, "metrics: duration between reports")
}

func startMetrics() {
	if ! flag.Parsed() {
		flag.Parse()
	}
	m.start()
}

func finalMetrics() {
	m.writeOnce()
}

func incr(name string, i int64) {
	m.incr(name, i)
}

func decr(name string, i int64) {
	m.decr(name, i)
}

func (m metrics) start() {
	go gometrics.WriteJSON(m.reg, m.tick, m.log)
}

func (m metrics) writeOnce() {
	gometrics.WriteJSONOnce(m.reg, m.log)
}

func (m metrics) incr(name string, i int64) {
	gometrics.GetOrRegisterCounter(name, m.reg).Inc(i)
}

func (m metrics) decr(name string, i int64) {
	gometrics.GetOrRegisterCounter(name, m.reg).Dec(i)
}
