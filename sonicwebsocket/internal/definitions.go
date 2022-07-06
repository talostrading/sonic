package internal

// operation defines the common interface that all websocket operations
// such as read/write/close etc. should implement.
type operation interface {
	ID() int
}
