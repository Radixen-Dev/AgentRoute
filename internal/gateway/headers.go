// SPDX-License-Identifier: GPL-3.0-only

package gateway

import "net/http"

// hopByHopHeaders are connection-scoped per RFC 7230 §6.1 and must never be
// forwarded between independent HTTP connections (here: the inbound
// tool<->gateway connection and the gateway<->upstream connection). Copying
// them verbatim corrupts framing — e.g. a leaked Content-Length from the
// upstream response can truncate or hang a response the gateway itself
// re-frames as chunked while streaming.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"Content-Length",
}

// copyResponseHeaders copies all headers from src to dst except the
// hop-by-hop set above. dst's writer (http.ResponseWriter) re-derives its
// own framing headers as the body is streamed.
func copyResponseHeaders(dst, src http.Header) {
	skip := make(map[string]struct{}, len(hopByHopHeaders))
	for _, h := range hopByHopHeaders {
		skip[h] = struct{}{}
	}
	for k, vv := range src {
		if _, ok := skip[http.CanonicalHeaderKey(k)]; ok {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
