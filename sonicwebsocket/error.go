package sonicwebsocket

import "errors"

var (
	// TODO these can be way more granular
	ErrCannotUpgrade         = errors.New("cannot upgrade to websocket")
	ErrReadingHeader         = errors.New("could not read header")
	ErrReadingExtendedLength = errors.New("could not read extended length")
	ErrReadingMask           = errors.New("could not read mask")
	ErrPayloadTooBig         = errors.New("payload too big")
)
