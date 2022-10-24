package http

import (
	"bytes"
	"fmt"
	"io"
)

type Proto string

const (
	ProtoHttp11 = "HTTP/1.1"
)

func ParseProtoFromBytes(b []byte) (Proto, error) {
	if bytes.Equal([]byte(ProtoHttp11), b) {
		return ProtoHttp11, nil
	}
	return "", fmt.Errorf("invalid or unsupported proto %s", string(b))
}

func (p Proto) String() string {
	return string(p)
}

type Method string

const (
	Get    Method = "GET"
	Post   Method = "POST"
	Head   Method = "HEAD"
	Put    Method = "PUT"
	Delete Method = "DELETE"
)

func ParseMethodFromBytes(b []byte) (Method, error) {
	if bytes.Equal([]byte(Get), b) {
		return Get, nil
	}
	if bytes.Equal([]byte(Post), b) {
		return Post, nil
	}
	if bytes.Equal([]byte(Head), b) {
		return Head, nil
	}
	if bytes.Equal([]byte(Put), b) {
		return Put, nil
	}
	if bytes.Equal([]byte(Delete), b) {
		return Delete, nil
	}
	return "", fmt.Errorf("invalid or unsupported method %s", string(b))
}

const CLRF = "\r\n"

func (m Method) String() string {
	return string(m)
}

type Header interface {
	io.WriterTo

	Add(key, value string)
	Set(key, value string)
	Get(key string) string
	Del(key string)
	Has(key string) bool
	Len() int
	Reset()
}
