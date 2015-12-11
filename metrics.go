package main

import (
	"fmt"
	gometrics "github.com/rcrowley/go-metrics"
	"net"
)

type metrics struct {
	reg  gometrics.Registry
}

var m = &metrics{reg: gometrics.DefaultRegistry}

func startMetrics() {
	ln, err := net.Listen("tcp", "127.0.0.1:8082")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go m.report(conn)
	}
}

func incr(name string, i int64) {
	m.incr(name, i)
}

func decr(name string, i int64) {
	m.decr(name, i)
}

func mark(name string, i int64) {
	m.mark(name, i)
}

func (m metrics) report(conn net.Conn) {
	defer conn.Close()
	m.reg.Each(func(name string, m interface{}) {
		switch m.(type) {
		case gometrics.Counter:
			fmt.Fprintf(conn, "%s.value %d\n", name, m.(gometrics.Counter).Count())
		case gometrics.Meter:
			fmt.Fprintf(conn, "%s5m.value %.3f\n", name, m.(gometrics.Meter).Rate5())
		}
	})
}

func (m metrics) incr(name string, i int64) {
	gometrics.GetOrRegisterCounter(name, m.reg).Inc(i)
}

func (m metrics) decr(name string, i int64) {
	gometrics.GetOrRegisterCounter(name, m.reg).Dec(i)
}

func (m metrics) mark(name string, i int64) {
	gometrics.GetOrRegisterMeter(name, m.reg).Mark(i)
}
