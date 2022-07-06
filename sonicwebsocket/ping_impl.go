package sonicwebsocket

func (s *WebsocketStream) Ping(b []byte) error {
	if len(b) > MaxControlFramePayloadSize {
		return ErrPayloadTooBig
	}
	return nil
}

func (s *WebsocketStream) AsyncPing(b []byte, cb func(error)) {
	if len(b) > MaxControlFramePayloadSize {
		cb(ErrPayloadTooBig)
	}
}
