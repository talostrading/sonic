package sonicwebsocket

func (s *WebsocketStream) Pong(b []byte) error {
	if len(b) > MaxControlFramePayloadSize {
		return ErrPayloadTooBig
	}
	return nil
}

func (s *WebsocketStream) AsyncPong(b []byte, cb func(error)) {
	if len(b) > MaxControlFramePayloadSize {
		cb(ErrPayloadTooBig)
	}
}
