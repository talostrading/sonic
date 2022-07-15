Note on state machines:
- `WebsocketStream` handles the state of the whole stream where the smallest unit is a frame.
- `FrameCodec` handles the state of a frame where the smallest unit is a byte.
- The `FrameCodec` state is owned by `WebsocketStream`. `FrameCodec` updates the state machine itself but the control on when it is reset is given to `WebsocketStream`. This is needed in order to reduce the number of byte slice copying between the internal websocket buffers and the user provided buffer.
