package main

import (
	"fmt"
	gometrics "github.com/rcrowley/go-metrics"
	"net"
	"sort"
)

type metrics struct {
	reg gometrics.Registry
}

var m = &metrics{reg: gometrics.DefaultRegistry}

func startMetrics(port string) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
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
	var names = []string{}
	var metrics = make(map[string]string)
	m.reg.Each(func(name string, m interface{}) {
		names = append(names, name)
		switch m.(type) {
		case gometrics.Counter:
			metrics[name] = fmt.Sprintf("%s.value %d\n", name, m.(gometrics.Counter).Count())
		case gometrics.Meter:
			metrics[name] = fmt.Sprintf("%s5m.value %.3f\n", name, m.(gometrics.Meter).Rate5())
		}
	})
	sort.Strings(names)
	for _, name := range names {
		conn.Write([]byte(metrics[name]))
	}
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
