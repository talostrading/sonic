package sonicwebsocket

import "fmt"

func (s *WebsocketStream) Accept() error {
	if s.role != RoleServer {
		return fmt.Errorf("invalid role=%s; only servers can accept", s.role)
	}

	// TODO
	return nil
}

func (s *WebsocketStream) AsyncAccept(cb func(error)) {
	if s.role != RoleServer {
		cb(fmt.Errorf("invalid role=%s; only servers can accept", s.role))
		return
	}
	// TODO
}
