package sonichttp

import "net/http"

type AsyncClientHandler func(error, *Client)
type AsyncResponseHandler func(error, *http.Response)
