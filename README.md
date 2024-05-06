# Noisy Sockets Shell

A WebSockets based remote shell written in Go.

## Why?

Because I wanted something akin to OpenSSH but with a simpler protocol that 
could be used directly in the browser.

## Design

* [WebSockets](https://en.wikipedia.org/wiki/WebSocket) (to support in-browser clients, with relatively low overhead for native clients).
* [JSON-RPC 2.0](https://www.jsonrpc.org/specification) inspired bidirectional messaging protocol (with message versioning).
* Single channel per connection (multiplexing over TCP isn't worth it due to [TCP head-of-line blocking](https://en.wikipedia.org/wiki/HTTP/2#TCP_head-of-line_blocking)).
* Only supports remote terminals (no port-forwarding, exec, etc). Keep it simple, stupid.