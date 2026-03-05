module github.com/Automattic/pinghub

go 1.26

require (
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
)

require golang.org/x/net v0.17.0 // indirect

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.9.3
