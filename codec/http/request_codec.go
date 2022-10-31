package http

import (
	"fmt"
	"github.com/talostrading/sonic"
	"strconv"
)

var (
	_ sonic.Encoder[*Request]         = &RequestEncoder{}
	_ sonic.Decoder[*Request]         = &RequestDecoder{}
	_ sonic.Codec[*Request, *Request] = &RequestCodec{}
)

type RequestEncoder struct {
}

func NewRequestEncoder() (*RequestEncoder, error) {
	enc := &RequestEncoder{}
	return enc, nil
}

func (enc *RequestEncoder) Encode(req *Request, dst *sonic.ByteBuffer) error {
	if err := ValidateRequest(req); err != nil {
		return err
	}

	// status-line
	if err := EncodeRequestLine(req, dst); err != nil {
		return err
	}

	// header
	if _, err := req.Header.WriteTo(dst); err != nil {
		return err
	}

	// body
	if ExpectBody(req.Header) {
		if n, err := dst.Write(req.Body); err != nil || n != len(req.Body) {
			return err
		}
	}

	return nil
}

type RequestDecoder struct {
	decodeState decodeState
	decodeReq   *Request // request we decode into
}

func NewRequestDecoder() (*RequestDecoder, error) {
	req, err := NewRequest()
	if err != nil {
		return nil, err
	}

	dec := &RequestDecoder{
		decodeState: stateFirstLine,
		decodeReq:   req,
	}
	return dec, nil
}

func (dec *RequestDecoder) resetDecode() {
	if dec.decodeState == stateDone {
		dec.decodeState = stateFirstLine
		dec.decodeReq.Reset()
	}
}

func (dec *RequestDecoder) Decode(src *sonic.ByteBuffer) (*Request, error) {
	dec.resetDecode()

	var (
		line []byte
		err  error

		headerKey, headerValue []byte

		n int64
	)

prepareDecode:
	if err != nil {
		goto done
	}

	switch s := dec.decodeState; s {
	case stateFirstLine:
		goto decodeFirstLine
	case stateHeader:
		goto decodeHeader
	case stateBody:
		goto decodeBody
	case stateDone:
		goto done
	default:
		panic(fmt.Errorf("unhandled state %d", s))
	}

decodeFirstLine:
	line, err = src.NextLine()
	if err == nil {
		err = DecodeRequestLine(line, dec.decodeReq)
	}

	if err == nil {
		dec.decodeState = stateHeader
	} else {
		dec.decodeState = stateDone
	}
	goto prepareDecode

decodeHeader:
	line, err = src.NextLine()
	if err == nil {
		if len(line) == 0 {
			// CLRF - end of header
			if ExpectBody(dec.decodeReq.Header) {
				dec.decodeState = stateBody
			} else {
				dec.decodeState = stateDone
			}
		} else {
			headerKey, headerValue, err = DecodeHeaderLine(line)
			if err == nil {
				dec.decodeReq.Header.Add(string(headerKey), string(headerValue))
			}
		}
	}
	goto prepareDecode

decodeBody:
	n, err = strconv.ParseInt(dec.decodeReq.Header.Get("Content-Length"), 10, 64)
	if err == nil {
		err = src.PrepareRead(int(n))
		if err == nil {
			dec.decodeReq.Body = src.Data()
			dec.decodeState = stateDone
		}
	}
	goto prepareDecode

done:
	return dec.decodeReq, err
}

type RequestCodec struct {
	*RequestEncoder
	*RequestDecoder
}

func NewRequestCodec() (*RequestCodec, error) {
	enc, err := NewRequestEncoder()
	if err != nil {
		return nil, err
	}

	dec, err := NewRequestDecoder()
	if err != nil {
		return nil, err
	}

	c := &RequestCodec{}
	c.RequestEncoder = enc
	c.RequestDecoder = dec

	return c, nil
}
