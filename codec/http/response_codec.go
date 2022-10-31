package http

import (
	"fmt"
	"github.com/talostrading/sonic"
	"strconv"
)

var (
	_ sonic.Encoder[*Response]          = &ResponseEncoder{}
	_ sonic.Decoder[*Response]          = &ResponseDecoder{}
	_ sonic.Codec[*Response, *Response] = &ResponseCodec{}
)

type ResponseEncoder struct{}

func NewResponseEncoder() (*ResponseEncoder, error) {
	enc := &ResponseEncoder{}
	return enc, nil
}

func (enc *ResponseEncoder) Encode(res *Response, dst *sonic.ByteBuffer) error {
	if err := ValidateResponse(res); err != nil {
		return err
	}

	// status-line
	if err := EncodeResponseLine(res, dst); err != nil {
		return err
	}

	// header
	if _, err := res.Header.WriteTo(dst); err != nil {
		return err
	}

	// body
	if ExpectBody(res.Header) {
		if n, err := dst.Write(res.Body); err != nil || n != len(res.Body) {
			return err
		}
	}

	return nil
}

type ResponseDecoder struct {
	decodeState decodeState
	decodeRes   *Response
}

func NewResponseDecoder() (*ResponseDecoder, error) {
	res, err := NewResponse()
	if err != nil {
		return nil, err
	}

	dec := &ResponseDecoder{
		decodeState: stateFirstLine,
		decodeRes:   res,
	}
	return dec, nil
}

func (dec *ResponseDecoder) resetDecode() {
	if dec.decodeState == stateDone {
		dec.decodeRes.Reset()
		dec.decodeState = stateFirstLine
	}
}

func (dec *ResponseDecoder) Decode(src *sonic.ByteBuffer) (*Response, error) {
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
		err = DecodeResponseLine(line, dec.decodeRes)
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
			if ExpectBody(dec.decodeRes.Header) {
				dec.decodeState = stateBody
			} else {
				dec.decodeState = stateDone
			}
		} else {
			headerKey, headerValue, err = DecodeHeaderLine(line)
			if err == nil {
				dec.decodeRes.Header.Add(string(headerKey), string(headerValue))
			}
		}
	}
	goto prepareDecode

decodeBody:
	n, err = strconv.ParseInt(dec.decodeRes.Header.Get("Content-Length"), 10, 64)
	if err == nil {
		err = src.PrepareRead(int(n))
		if err == nil {
			dec.decodeRes.Body = src.Data()
			dec.decodeState = stateDone
		}
	}
	goto prepareDecode

done:
	return dec.decodeRes, err
}

type ResponseCodec struct {
	*ResponseEncoder
	*ResponseDecoder
}

func NewResponseCodec() (*ResponseCodec, error) {
	enc, err := NewResponseEncoder()
	if err != nil {
		return nil, err
	}

	dec, err := NewResponseDecoder()
	if err != nil {
		return nil, err
	}

	c := &ResponseCodec{}
	c.ResponseEncoder = enc
	c.ResponseDecoder = dec

	return c, nil
}
