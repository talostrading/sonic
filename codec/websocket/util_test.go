package websocket

import "testing"

func TestCloseFramePayloadCodec(t *testing.T) {
	{
		var b []byte = nil
		code, reason := DecodeCloseFramePayload(b)
		if !(code == CloseNoStatus && reason == "") {
			t.Fatal("did not handle empty close frame correctly")
		}
	}
	{
		b := make([]byte, 1)
		code, reason := DecodeCloseFramePayload(b)
		if !(code == CloseNoStatus && reason == "") {
			t.Fatal("did not handle empty close frame correctly")
		}
	}
	{
		b := EncodeCloseFramePayload(CloseNormal, "")
		if len(b) != 2 {
			t.Fatal("did not encode the close frame correctly")
		}

		code, reason := DecodeCloseFramePayload(b)
		if !(code == CloseNormal && reason == "") {
			t.Fatal("did not handle close frame without reason correctly")
		}
	}
	{
		b := EncodeCloseFramePayload(CloseNormal, "something")
		if len(b) != 2+len("something") {
			t.Fatal("did not encode the close frame correctly")
		}

		code, reason := DecodeCloseFramePayload(b)
		if !(code == CloseNormal && reason == "something") {
			t.Fatal("did not handle close frame with reason correctly")
		}
	}
}
