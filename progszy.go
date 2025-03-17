// Package progszy is a hard-caching HTTP(S) proxy server, using SQLite & Zstd.
package progszy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	retryablehttp "github.com/hashicorp/go-retryablehttp"

	"github.com/elazarl/goproxy"
)

// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Caching

// TODO(js) We have a subtle issue here: one of the sites has a sitemap of almost 300mb.
// The spec says it shouldn't be more than 50mb, but it's difficult to argue with the reality of the situation.

// maxBodySize is the maximum number of bytes to read from the response body.
const maxBodySize = 512 * 1024 * 1024 // 512mb

// const maxBodySize = 128 * 1024 * 1024 // 128mb
// const maxBodySize = 16 * 1024 * 1024 // 16mb
// const maxBodySize = 1 * 1024 * 1024 // 1mb

func ProxyHandlerWith(cache Cache, proxy *url.URL) http.Handler {

	p := goproxy.NewProxyHttpServer()
	// TODO Control goproxy logging from outside.
	// proxy.Verbose = true
	p.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	handler := proxyHandler(cache, proxy)

	p.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		return nil, handler(req)
	})

	return p
}

// TODO Arguably we should implement some kind of ResponseWriter, instead of manually building the response?

// ----------------------------

func proxyHandler(cache Cache, proxy *url.URL) func(*http.Request) *http.Response {

	// Parse incoming HTTP request.
	// Get requested URL.
	// Is it in the cache?
	// Yes, return cached response.
	//
	// No, request URL from upstream.
	// If HTTP error, return error.
	// If rule-based error, return error.
	// Store response in cache.
	// Return response.

	handleCacheMiss := makeCacheMissHandler(proxy)

	return func(r *http.Request) *http.Response {

		start := time.Now()
		defer func() {
			dur := time.Since(start)
			log.Printf("total handler duration %.3fms", float64(dur)/float64(time.Millisecond))
		}()

		// TODO Better error handling throughout.

		// dump, err := httputil.DumpRequest(r, true)
		// if err != nil {
		// 	// http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		// 	// return
		// 	return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		// }
		// fmt.Println(string(dump))
		// // fmt.Fprintf(w, "%q", dump)
		// // fmt.Printf("%q", dump)

		// fmt.Printf("====== headers\n%v", r.Header)

		// We only handle GET & HEAD requests for now.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			m := fmt.Sprintf("Method not allowed (%s)", r.Method)
			return httpError(r, m, http.StatusMethodNotAllowed)
		}

		// Consume the request body.
		io.Copy(io.Discard, r.Body)
		r.Body.Close()

		uri := r.RequestURI
		// fmt.Println("RequestURI: " + uri)

		host := r.Host
		if len(host) > 0 {
			// log.Printf("============host uri %s", host)
			path, err := url.Parse(uri)
			if err != nil {
				m := fmt.Sprintf("Path parse error %s", uri)
				return httpError(r, m, http.StatusBadRequest)
			}
			base, err := url.Parse("https://" + host)
			if err != nil {
				m := fmt.Sprintf("Host parse error %s", host)
				return httpError(r, m, http.StatusBadRequest)
			}
			uri = base.ResolveReference(path).String()
		}

		// log.Printf("============requested uri %s", uri)

		if r.Header.Get("X-Cache-Flush") == "TRUE" {
			err := cache.Flush(uri)
			if err != nil {
				m := fmt.Sprintf("Cache flush error %s", err)
				return httpError(r, m, http.StatusBadRequest)
			}
			resp := newResponse(r, http.StatusOK)
			resp.Header.Set("X-Cache", "FLUSHED")
			return resp
		}

		// Try to get from cache.
		cr, err := cache.Get(uri)
		if err == nil {
			// Cache hit.
			// log.Println("cache hit")

			resp := newResponse(r, http.StatusOK)
			resp.Header.Set("X-Cache", "HIT")
			applyCommonHeaders(resp, cr)

			switch r.Method {
			case http.MethodGet:
				resp.Body, err = cr.Body()
				if err != nil {
					log.Printf("Cache body error during GET: %v\n", err)
					return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
				}
				log.Printf("decompressed content size %s", byteCountDecimal(cr.ContentLength))
			case http.MethodHead:
				// No action.
			}
			return resp
		}
		if err != ErrCacheMiss {
			log.Printf("cache.Get error: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}

		return handleCacheMiss(r, uri, cache)
	}
}

func makeCacheMissHandler(proxy *url.URL) func(r *http.Request, uri string, cache Cache) *http.Response {

	rulesCache := newRulesMap()
	secureClient := newClient(false, proxy)
	insecureClient := newClient(true, proxy)

	return func(r *http.Request, uri string, cache Cache) *http.Response {

		// Cache miss - fetch and cache.

		resp := newResponse(r, http.StatusOK)
		resp.Header.Set("X-Cache", "MISS")

		// log.Println("cache miss")

		// Build the request.
		req, err := retryablehttp.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			log.Printf("retryablehttp.NewRequest error: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}
		copyHeaders(req.Header, r.Header)

		// log.Printf("Outgoing request URL: %s\n", uri)
		// log.Printf("Outgoing headers: %v\n", req.Header)

		// Get appropriately configured client.
		client := secureClient
		if r.Header.Get("X-Cache-SSL") == "INSECURE" {
			client = insecureClient
		}
		// Do the request.
		rstart := time.Now()
		response, err := client.Do(req)
		if err != nil {
			log.Printf("client.Do error: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}
		defer response.Body.Close()

		rend := time.Now()
		rdur := rend.Sub(rstart)
		responseTime := float64(rdur) / float64(time.Millisecond)

		// TODO Should we check content type is text/HTML/JSON/CSS (not binary data) ?

		// Read the response body, limiting the max size.
		lr := io.LimitedReader{R: response.Body, N: maxBodySize + 1}
		body, err := io.ReadAll(&lr)
		if err != nil {
			// TODO(js) This has failed before. Can we retry somehow?
			log.Printf("ioutil.ReadAll error: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}
		if lr.N == 0 {
			// Exceeded max body size.
			io.Copy(io.Discard, response.Body)
			max := byteCountDecimal(maxBodySize)
			m := fmt.Sprintf("Body exceeds maximum size (%s)", max)
			log.Println(m)
			return httpError(r, m, http.StatusInsufficientStorage)
		}
		log.Printf("upstream request duration %.3fms", float64(time.Since(rstart))/float64(time.Millisecond))

		// Check status code is good - we only accept 200 ok (the client handles redirects).
		if response.StatusCode != 200 {
			// Upstream error.
			// TODO We could return the original status code + body? No...
			// TODO Should we return a 500 here - we only handle 200.
			m := fmt.Sprintf("Upstream server returned status %s - %s", response.Status, http.StatusText(response.StatusCode))
			log.Println(m)
			return httpError(r, m, response.StatusCode)
		}

		// Check page body against reject rules.

		// TODO(js) Time stats for creation/compilation of regex rules.

		// TODO(js) Note: if a page already exists in the cache, reject rules are not applied. Document this.
		// TODO(js) But perhaps reject rules should be applied there also? Perhaps causing a resource to be evicted from the cache?
		// TODO(js) Document this.

		rejectRulesHeaders := r.Header["X-Cache-Reject"]
		// log.Printf("Reject rules %v", rejectRulesHeaders)

		rules, err := rulesCache.getAll(rejectRulesHeaders)
		if err != nil {
			m := fmt.Sprintf("Unable to compile X-Cache-Reject pattern: %v", err)
			log.Println(m)
			return httpError(r, m, http.StatusInternalServerError)
		}

		for _, re := range rules {
			// Abort the request if any rule matches.
			if re.Match(body) {
				m := fmt.Sprintf("Content rejected by match: %s", re.String())
				return httpError(r, m, http.StatusPreconditionFailed)
			}
		}

		// Get metadata.
		status := response.StatusCode
		proto := response.Proto
		lang := response.Header.Get("Content-Language")
		mime := response.Header.Get("Content-Type")
		etag := response.Header.Get("ETag")
		lastMod := response.Header.Get("Last-Modified")

		// Put asset in the cache.
		cr, err := NewCacheRecord(uri, status, proto, lang, mime, etag, lastMod, body, responseTime, rend)
		if err != nil {
			log.Printf("Error creating CacheRecord: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}
		err = cache.Put(cr)
		if err != nil {
			log.Printf("cache.Put error: %v\n", err)
			return httpError(r, fmt.Sprint(err), http.StatusInternalServerError)
		}
		// log.Printf("cached content size %s", byteCountDecimal(int64(len(body))))

		// Finally, send to client.
		applyCommonHeaders(resp, cr)
		switch r.Method {
		case "GET":
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
		case "HEAD":
			// No action.
		}
		return resp
	}
}

func applyCommonHeaders(resp *http.Response, cr *CacheRecord) {
	// We force UTC for X-Cache-Timestamp here,
	// so that old cache dbs (created before today, 11-Aug-2020)
	// will still present times as UTC.
	resp.Header.Set("X-Cache-Timestamp", cr.Created.UTC().Format(time.RFC3339Nano))
	resp.Header.Set("Content-Length", strconv.Itoa(int(cr.ContentLength)))
	if len(cr.ContentType) > 0 {
		resp.Header.Set("Content-Type", cr.ContentType)
	}
	if len(cr.ETag) > 0 {
		resp.Header.Set("ETag", cr.ETag)
	}
	if len(cr.LastModified) > 0 {
		resp.Header.Set("Last-Modified", cr.LastModified)
	}
	if len(cr.ContentLanguage) > 0 {
		resp.Header.Set("Content-Language", cr.ContentLanguage)
	}
}

func newResponse(r *http.Request /*contentType string,*/, status int) *http.Response {
	resp := &http.Response{}
	resp.Request = r
	resp.TransferEncoding = r.TransferEncoding
	resp.Header = make(http.Header)
	// resp.Header.Add("Content-Type", contentType)
	resp.StatusCode = status
	resp.Status = http.StatusText(status)
	// buf := bytes.NewBufferString(body)
	// resp.ContentLength = int64(buf.Len())
	// resp.Body = ioutil.NopCloser(buf)
	// resp.Body = ioutil.NopCloser(body)
	resp.Body = io.NopCloser(&bytes.Buffer{})
	return resp
}

func newResponseWithBody(r *http.Request /*contentType string,*/, status int, body io.Reader) *http.Response {
	resp := newResponse(r, status)
	resp.Body = io.NopCloser(body)
	return resp
}

func httpError(r *http.Request, message string, status int) *http.Response {
	body := io.NopCloser(bytes.NewBufferString(message))
	return newResponseWithBody(r, status, body)
}

// // From goproxy
// //
// func NewResponse(r *http.Request, contentType string, status int, body string) *http.Response {
// 	resp := &http.Response{}
// 	resp.Request = r
// 	resp.TransferEncoding = r.TransferEncoding
// 	resp.Header = make(http.Header)
// 	resp.Header.Add("Content-Type", contentType)
// 	resp.StatusCode = status
// 	resp.Status = http.StatusText(status)
// 	buf := bytes.NewBufferString(body)
// 	resp.ContentLength = int64(buf.Len())
// 	resp.Body = ioutil.NopCloser(buf)
// 	return resp
// }

// Default retry configuration

var (
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
	defaultRetryMax     = 4
)

var acceptAllCerts = &tls.Config{InsecureSkipVerify: true}

func newClient(insecure bool, proxy *url.URL) *retryablehttp.Client {
	// TODO Client configuration - see https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779

	// TODO Note that because we use a retrying client, this means outgoing HTTP requests can now take a longer time.
	// Do we need to make the HTTP server and the requesting client have longer timeouts to handle this? Review this.

	// TODO(js) Now we make our own client, how to (optionally) enable logging again?

	client := &retryablehttp.Client{
		HTTPClient: cleanhttp.DefaultPooledClient(),
		// Logger:       defaultLogger,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}
	// client.Logger = nil

	// tr := &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: acceptAllCerts}

	tr := client.HTTPClient.Transport.(*http.Transport)
	if insecure {
		tr.TLSClientConfig = acceptAllCerts
	}
	if proxy != nil {
		tr.Proxy = http.ProxyURL(proxy)
	}

	return client
}

func byteCountDecimal(b int64) string {
	// From https://programming.guide/go/formatting-byte-size-to-human-readable-format.html
	// With minor format tweak.
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func copyHeaders(dst, src http.Header) {
	// See also makeHeadersCopier in https://golang.org/src/net/http/client.go
	for k, vv := range src {
		if shouldCopyHeader(k) {
			for _, v := range vv {
				dst.Add(k, v)
				// log.Printf("Copying header: %s = %s\n", k, v)
			}
		}
	}
}

func shouldCopyHeader(headerKey string) bool {
	key := http.CanonicalHeaderKey(headerKey)
	switch key {
	case "Accept-Encoding":
		// http.Client handles this itself.
		// If we copy it across, and it says gzip (it will do),
		// then we have to manually handle gzip decoding.
		return false
	}

	// TODO(js) Perhaps we should have a more precise filter, for our specific X- headers? Arguably, it's more complex and harder to maintain. So let's leave this unless it causes an issue.

	// We copy remaining keys, if they are not special control headers.
	return !strings.HasPrefix(key, "X-")
}
