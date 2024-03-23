## Compliance

`sonic.websocket` uses the [autobahn-testsuite](https://github.com/crossbario/autobahn-testsuite) to validate the
WebSocket implementation. `sonic.websocket` implements most of the WebSocket protocol with the exception of:

- DEFLATE compression
- UTF8 handling.

## Notes

There are two state machines that combined form a stateful WebSocket parser.

- `FrameCodec` handles the state of a frame where the smallest unit is a byte.
- `WebsocketStream` handles the state of the whole stream where the smallest unit is a frame. `WebsocketStream`
  uses `FrameCodec`.
