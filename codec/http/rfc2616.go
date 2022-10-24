package http

// HTTP Status-Codes
// https://www.iana.org/assignments/http-status-codes/http-status-codes.xhtml

type Status int

const (
	StatusContinue           Status = 100
	StatusSwitchingProtocols Status = 101

	StatusOK                   Status = 200
	StatusCreated              Status = 201
	StatusAccepted             Status = 202
	StatusNonAuthoritativeInfo Status = 203
	StatusNoContent            Status = 204
	StatusResetContent         Status = 205
	StatusPartialContent       Status = 206

	StatusMultipleChoices   Status = 300
	StatusMovedPermanently  Status = 301
	StatusFound             Status = 302
	StatusSeeOther          Status = 303
	StatusNotModified       Status = 304
	StatusUseProxy          Status = 305
	StatusTemporaryRedirect Status = 307

	StatusBadRequest                   Status = 400
	StatusUnauthorized                 Status = 401
	StatusPaymentRequired              Status = 402
	StatusForbidden                    Status = 403
	StatusNotFound                     Status = 404
	StatusMethodNotAllowed             Status = 405
	StatusNotAcceptable                Status = 406
	StatusProxyAuthRequired            Status = 407
	StatusRequestTimeout               Status = 408
	StatusConflict                     Status = 409
	StatusGone                         Status = 410
	StatusLengthRequired               Status = 411
	StatusPreconditionFailed           Status = 412
	StatusRequestEntityTooLarge        Status = 413
	StatusRequestURITooLong            Status = 414
	StatusUnsupportedMediaType         Status = 415
	StatusRequestedRangeNotSatisfiable Status = 416
	StatusExpectationFailed            Status = 417

	StatusInternalServerError     Status = 500
	StatusNotImplemented          Status = 501
	StatusBadGateway              Status = 502
	StatusServiceUnavailable      Status = 503
	StatusGatewayTimeout          Status = 504
	StatusHTTPVersionNotSupported Status = 505
)

func (s Status) String() string {
	switch s {
	case StatusContinue:
		return "Continue"
	case StatusSwitchingProtocols:
		return "Switching Protocols"

	case StatusOK:
		return "OK"
	case StatusCreated:
		return "Created"
	case StatusAccepted:
		return "Accepted"
	case StatusNonAuthoritativeInfo:
		return "Non-Authoritative Information"
	case StatusNoContent:
		return "No Content"
	case StatusResetContent:
		return "Reset Content"
	case StatusPartialContent:
		return "Partial Content"

	case StatusMultipleChoices:
		return "Multiple Choices"
	case StatusMovedPermanently:
		return "Moved Permanently"
	case StatusFound:
		return "Found"
	case StatusSeeOther:
		return "See Other"
	case StatusNotModified:
		return "Not Modified"
	case StatusUseProxy:
		return "Use Proxy"
	case StatusTemporaryRedirect:
		return "Temporary Redirect"

	case StatusBadRequest:
		return "Bad Request"
	case StatusUnauthorized:
		return "Unauthorized"
	case StatusPaymentRequired:
		return "Payment Required"
	case StatusForbidden:
		return "Forbidden"
	case StatusNotFound:
		return "Not Found"
	case StatusMethodNotAllowed:
		return "Method Not Allowed"
	case StatusNotAcceptable:
		return "Not Acceptable"
	case StatusProxyAuthRequired:
		return "Proxy Authentication Required"
	case StatusRequestTimeout:
		return "Request Timeout"
	case StatusConflict:
		return "Conflict"
	case StatusGone:
		return "Gone"
	case StatusLengthRequired:
		return "Length Required"
	case StatusPreconditionFailed:
		return "Precondition Failed"
	case StatusRequestEntityTooLarge:
		return "Request Entity Too Large"
	case StatusRequestURITooLong:
		return "Request URI Too Long"
	case StatusUnsupportedMediaType:
		return "Unsupported Media Type"
	case StatusRequestedRangeNotSatisfiable:
		return "Requested Range Not Satisfiable"
	case StatusExpectationFailed:
		return "Expectation Failed"

	case StatusInternalServerError:
		return "Internal Server Error"
	case StatusNotImplemented:
		return "Not Implemented"
	case StatusBadGateway:
		return "Bad Gateway"
	case StatusServiceUnavailable:
		return "Service Unavailable"
	case StatusGatewayTimeout:
		return "Gateway Timeout"
	case StatusHTTPVersionNotSupported:
		return "HTTP Version Not Supported"

	default:
		return "idk"
	}
}
