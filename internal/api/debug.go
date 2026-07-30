package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// debugBodyCap bounds how many bytes of any request or response body are
// dumped. Anything beyond is replaced with a "(... bytes elided)" marker.
const debugBodyCap = 64 * 1024

// debugTransport wraps an http.RoundTripper and writes a human-readable
// summary of each request/response pair to w. It is safe for concurrent
// use insofar as the underlying transport is.
type debugTransport struct {
	base http.RoundTripper
	w    io.Writer
}

func newDebugTransport(base http.RoundTripper, w io.Writer) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &debugTransport{base: base, w: w}
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d.dumpRequest(req)
	resp, err := d.base.RoundTrip(req)
	if err != nil {
		_, _ = fmt.Fprintf(d.w, "< (transport error: %v)\n\n", err)
		return resp, err
	}
	d.dumpResponse(resp)
	return resp, nil
}

func (d *debugTransport) dumpRequest(req *http.Request) {
	reqURI := req.URL.RequestURI()
	proto := req.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	_, _ = fmt.Fprintf(d.w, "> %s %s %s\n", req.Method, reqURI, proto)
	if req.Host != "" {
		_, _ = fmt.Fprintf(d.w, "> Host: %s\n", req.Host)
	} else if req.URL.Host != "" {
		_, _ = fmt.Fprintf(d.w, "> Host: %s\n", req.URL.Host)
	}
	writeHeaders(d.w, "> ", req.Header)

	body, restored, err := drainBody(req.Body)
	if err != nil {
		_, _ = fmt.Fprintf(d.w, "> (error reading body: %v)\n\n", err)
		return
	}
	if restored != nil {
		req.Body = restored
	}
	writeBody(d.w, "> ", req.Header.Get("Content-Type"), body)
}

func (d *debugTransport) dumpResponse(resp *http.Response) {
	proto := resp.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	_, _ = fmt.Fprintf(d.w, "< %s %s\n", proto, resp.Status)
	writeHeaders(d.w, "< ", resp.Header)

	body, restored, err := drainBody(resp.Body)
	if err != nil {
		_, _ = fmt.Fprintf(d.w, "< (error reading body: %v)\n\n", err)
		return
	}
	if restored != nil {
		resp.Body = restored
	}
	writeBody(d.w, "< ", resp.Header.Get("Content-Type"), body)
}

// writeHeaders prints headers in sorted key order with each line prefixed
// by p. Authorization and Cookie values are redacted.
func writeHeaders(w io.Writer, p string, h http.Header) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			_, _ = fmt.Fprintf(w, "%s%s: %s\n", p, k, redactHeader(k, v))
		}
	}
	_, _ = fmt.Fprintf(w, "%s\n", strings.TrimRight(p, " "))
}

// redactHeader masks sensitive header values. For Authorization, a Bearer
// token long enough to keep useful prefix/suffix bytes shows "Bearer
// xxxx***yyyy"; shorter values collapse to "Bearer ***".
func redactHeader(name, value string) string {
	switch http.CanonicalHeaderKey(name) {
	case "Authorization":
		const scheme = "Bearer "
		if strings.HasPrefix(value, scheme) {
			tok := strings.TrimPrefix(value, scheme)
			if len(tok) >= 12 {
				return scheme + tok[:4] + "***" + tok[len(tok)-4:]
			}
			return scheme + "***"
		}
		return "***"
	case "Cookie", "Set-Cookie":
		return "***"
	}
	return value
}

// writeBody prints the body section after headers. JSON-ish bodies are
// dumped verbatim up to debugBodyCap; non-JSON bodies are summarized.
func writeBody(w io.Writer, p, contentType string, body []byte) {
	if len(body) == 0 {
		_, _ = fmt.Fprintf(w, "%s(no body)\n\n", p)
		return
	}
	if !isPrintableTextContentType(contentType) {
		ct := contentType
		if ct == "" {
			ct = "unknown content-type"
		}
		_, _ = fmt.Fprintf(w, "%s(%d bytes of %s)\n\n", p, len(body), ct)
		return
	}
	if len(body) > debugBodyCap {
		_, _ = w.Write([]byte(p))
		_, _ = w.Write(body[:debugBodyCap])
		if !bytes.HasSuffix(body[:debugBodyCap], []byte("\n")) {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintf(w, "%s... (%d bytes elided)\n\n", p, len(body)-debugBodyCap)
		return
	}
	_, _ = w.Write([]byte(p))
	_, _ = w.Write(body)
	if !bytes.HasSuffix(body, []byte("\n")) {
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w)
}

// isPrintableTextContentType reports whether a body of this Content-Type
// is safe to dump verbatim. JSON, text/*, and form-encoded payloads
// qualify; everything else (octet streams, binary uploads) is summarized.
func isPrintableTextContentType(ct string) bool {
	mt := strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	switch {
	case mt == "":
		return false
	case mt == "application/json", strings.HasSuffix(mt, "+json"):
		return true
	case strings.HasPrefix(mt, "text/"):
		return true
	case mt == "application/xml", strings.HasSuffix(mt, "+xml"):
		return true
	case mt == "application/x-www-form-urlencoded":
		return true
	}
	return false
}

// drainBody reads body to memory and returns a fresh ReadCloser that
// replays the same bytes, so the transport can still send (or the caller
// can still read) the original payload.
func drainBody(body io.ReadCloser) ([]byte, io.ReadCloser, error) {
	if body == nil || body == http.NoBody {
		return nil, body, nil
	}
	buf, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("drain body: %w", err)
	}
	return buf, io.NopCloser(bytes.NewReader(buf)), nil
}
