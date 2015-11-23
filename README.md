# Pinghub
Pinghub is a simple pubsub message bus for websocket clients. It also accepts messages by POST. Scale simply by placing several Pinghub servers behind a reverse proxy with consistent hashing of the request URI. The reverse proxy can also implement TLS and auth if needed.

### Command
```
pinghub -addr=:8081 -origin=https://ws.example.com
```

`-addr` sets the TCP listen address ([net.Dial](https://golang.org/pkg/net/#Dial)). Default: `-addr=:8081`

`-origin` enables checking the Origin header before starting the websocket handshake. Default: off.

### Security
Pinghub validates Origin headers if started with the `-origin` option. Secure transport, authentication and authorization can be implemented by a reverse proxy or load balancer placed between clients and servers.

### Protocol
The service was designed to provide a simple mechanism to push updates to browsers instead of making them poll for changes. Web clients subscribe for updates; application servers POST them.

Web clients can also publish. This provides a method of inter-client communication.

#### Messages
A message is a UTF-8 string transmitted in a websocket text frame. After UTF-8 validation, Pinghub ignores the content of messages and forgets them once delivered; no history is kept.

#### Clients
A websocket client subscribes by connecting a websocket to any valid UTF-8 path on the server.

A websocket client publishes by sending a message to the server.

A non-websocket client publishes by POSTing a request body to any path on the Pinghub server. It can not subscribe.

#### Channels
A channel is a FIFO queue which broadcasts each message to each subscriber. Channels are created on demand and closed when their last subscriber leaves.

This means all subscribers receive all messages in the same order.

A channel exists only while at least one websocket client is connected. A message POSTed to a path with no subscribers is dropped.

### Reverse Proxy
This Nginx config snippit sets up a websocket proxy. Requests for `//api.example.com/pinghub/*` are forwarded to an upstream server selected by a hash of the request URI, ensuring all clients for a given path are connected to the same server.
```
map $http_upgrade $connection_upgrade {
  default Upgrade;
  '' close;
}

upstream pinghub {
  hash $request_uri;
  server pinghub1.example.com:8081;
  server pinghub2.example.com:8081;
}

server {
  server_name api.example.com;
  ...
  location /pinghub/ {
    proxy_pass http://upstream;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
  }
}
```
