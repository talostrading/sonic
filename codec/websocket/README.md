Note on state machines:
- `WebsocketStream` handles the state of the whole stream where the smallest unit is a frame.
- `FrameCodec` handles the state of a frame where the smallest unit is a byte.
