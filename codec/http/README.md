This is a very barebones HTTP codec which doesn't implement most of the RFC. Use this at your own risk.

It currently exposes a client which can take an http request, serialize it and then read and deserialize the server's response. If on a second request the underlying TCP connection is closed, the client reconnects and sends the request.
