package websocket

import "errors"

var (
	ErrPayloadOverMaxSize = errors.New("payload over maximum size")

	ErrPayloadTooBig = errors.New("frame payload too big")

	ErrWrongHandshakeRole = errors.New(
		"wrong role when initiating/accepting the handshake",
	)

	ErrCannotUpgrade = errors.New(
		"cannot upgrade connection to WebSocket",
	)

	ErrMessageTooBig = errors.New("message too big")

	ErrInvalidControlFrame = errors.New("invalid control frame")

	ErrControlFrameTooBig = errors.New("control frame too big")

	ErrSendAfterClose = errors.New("sending on a closed stream")

	ErrNonZeroReservedBits = errors.New("non zero reserved bits")

	ErrMaskedFramesFromServer = errors.New("masked frames from server")

	ErrUnmaskedFramesFromClient = errors.New("unmasked frames from server")

	ErrReservedOpcode = errors.New("reserved opcode")

	ErrUnexpectedContinuation = errors.New(
		"continue frame but nothing to continue",
	)

	ErrExpectedContinuation = errors.New("expected continue frame")

	ErrInvalidAddress = errors.New("invalid address")

	ErrInvalidUTF8 = errors.New("Invalid UTF-8 encoding")
)
