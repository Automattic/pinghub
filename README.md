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

Web clients can also publish. This provides a method of inter-client communication. Any non-empty message from a client will be broadcast to the channel.

Empty messages are dropped by the server and not broadcast. Therefore clients can use empty messages as keepalive signals. The `proxy_read_timeout` nginx directive enforces this by disconnecting clients that fail to send messages.

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
upstream _pinghub {
  hash $request_uri;
  server pinghub1.example.com:8081;
  server pinghub2.example.com:8081;
}

server {
  server_name api.example.com;
  # ... (listen, ssl, etc.)

  location ^~ /pinghub {
    # expect pinghub to respond quickly
    proxy_send_timeout 5s;
    # disconnect idle clients (keepalive enforcement)
    proxy_read_timeout 95s;
    # send all requests upstream with selected headers
    proxy_pass http://_pinghub;
    proxy_pass_request_headers off;
    proxy_set_header Origin $http_origin;
    proxy_set_header Host $http_host;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $http_connection;
    proxy_set_header Sec-WebSocket-Extensions $http_sec_websocket_extensions;
    proxy_set_header Sec-WebSocket-Version $http_sec_websocket_version;
    proxy_set_header Sec-WebSocket-Key $http_sec_websocket_key;
  }

  # ... (other locations, etc.)
}
```

### Check Origin and Auth in Nginx
To keep Pinghub secure you can put it behind a reverse proxy that enforces encryption, origin and auth. This is easily done with [lua](https://github.com/openresty/lua-nginx-module) and your own backend auth service. The idea is to perform a subrequest to check the request against a separate API before forwarding the request to Pinghub. In the following example config we have an `internal-auth` API that:

* receives the original headers, plus X-Original-URI and X-Original-Method, in an HTTP request from nginx
* for requests with an Origin header, checks it against a whitelist
* authenticates the user by Cookie or other auth header or URI parameter
* authorizes access by comparing the user to the path in X-Original-URI
* rewrites the path if necessary
* responds with status codes and X-Rewrite header recognized by the lua block

```
  location ^~ /pinghub {
    error_page 403 = @autherror;
    access_by_lua_block {
      local res = ngx.location.capture("/__auth")
      if res.status == 204 then
        if res.header["X-Rewrite"] then
          ngx.req.set_uri(res.header["X-Rewrite"])
        end
        return
      end
      if res.status == 403 then
        ngx.exit(res.status)
      end
      ngx.exit(ngx.HTTP_INTERNAL_SERVER_ERROR)
    }
    ...
  }

  location /__auth {
    internal;
    error_page 403 = @autherror;
    proxy_pass http://127.0.0.1:8089/internal-auth/;
    proxy_http_version 1.1;
    proxy_intercept_errors off;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header Host api.example.com;
    proxy_set_header X-Original-URI $request_uri;
    proxy_set_header X-Original-Method $request_method;
  }

  location @autherror {
    default_type text/plain;
    return 403 'Forbidden';
  }
```

This `internal-auth` endpoint uses `204` to signal success. `200` is disallowed because it is a common default response code. A simple misconfiguration, such as a PHP server that returns `200` on fatal errors, could wrongly allow connections to private channels. So it is prudent to use a different success signal. `204` is convenient but you might want to use something even harder to mistake, such as a response header containing a secret success code.

The X-Rewrite trick can be used when clients don't know exactly what path to use. This has been used to support authenticated clients that don't know their own identities. They need to subscribe to `/pinghub/user/ID/chan` but they can't read the user ID encapsulated in their auth cookie. The client requests `/pinghub/me/chan` and ultimately connects to `/pinghub/user/157/chan`. Neither Pinghub nor the client knows that the request line has been rewritten by Nginx.
