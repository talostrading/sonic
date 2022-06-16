package sonicwebsocket

import "errors"

var (
	ErrCannotUpgrade         = errors.New("cannot upgrade to websocket")
	ErrReadingHeader         = errors.New("could not read header")
	ErrReadingExtendedLength = errors.New("could not read extended length")
	ErrReadingMask           = errors.New("could not read mask")
	ErrPayloadTooBig         = errors.New("payload too big")
	ErrInvalidControlFrame   = errors.New("invalid control frame")
)
