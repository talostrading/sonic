package sonicwebsocket

import "errors"

var (
	ErrCannotUpgrade           = errors.New("cannot upgrade to websocket")
	ErrReadingHeader           = errors.New("could not read header")
	ErrReadingExtendedLength   = errors.New("could not read extended length")
	ErrReadingMask             = errors.New("could not read mask")
	ErrPayloadTooBig           = errors.New("payload too big")
	ErrInvalidControlFrame     = errors.New("invalid control frame")
	ErrOnClose                 = errors.New("error on close")
	ErrCloseWhileHandshaking   = errors.New("closing while in handshake")
	ErrOperationAborted        = errors.New("operation aborted")
	ErrFragmentedControlFrame  = errors.New("fragmented control frame")
	ErrControlFrameTooBig      = errors.New("control frame length is over 125 bytes")
	ErrUnknownFrameType        = errors.New("unknown frame type")
	ErrSendAfterClosing        = errors.New("send after closing")
	ErrInvalidFrame            = errors.New("invalid frame")
	ErrMaskedFrameFromServer   = errors.New("masked frame from server")
	ErrUnmaskedFrameFromClient = errors.New("unmasked frame from client")
)
