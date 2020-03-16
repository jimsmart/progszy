// Package progszy is a caching HTTP proxy service, using SQLite & Zstd.
package progszy

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Caching

// Create an HTTP client - strategy? shared? one per server/domain?

// Start an HTTP listener.
// Log all requests.

// Parse incoming HTTP request.
// Get requested URL.
// Is it in the cache?
// Yes, return cached response.
// No, request URL from upstream.
// If HTTP error, return error.
// If rule-based error, return error.
// Store response in cache.
// Return response.

// func Run() {
// 	fmt.Println("hello")
// }

// TODO(js) We have a subtle issue here: one of the sites has a sitemap of almost 300mb.
// The spec says it shouldn't be more than 50mb, but it's difficult to argue with the reality of the situation.

// maxBodySize is the maximum number of bytes to read from the response body.
const maxBodySize = 64 * 1024 * 1024 // 64mb

// const maxBodySize = 16 * 1024 * 1024 // 16mb
// const maxBodySize = 1 * 1024 * 1024 // 1mb

func ProxyHandlerWith(cache Cache) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		// TODO We do not handle ETag header. See https://www.keycdn.com/blog/http-cache-headers
		// TODO We do not handle Last-Modified header.

		start := time.Now()
		defer func() {
			dur := time.Since(start)
			log.Printf("total request duration %v", dur)
		}()

		// TODO Better error handling throughout.

		// dump, err := httputil.DumpRequest(r, true)
		// if err != nil {
		// 	http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
		// 	return
		// }
		// fmt.Println(string(dump))
		// // fmt.Fprintf(w, "%q", dump)
		// // fmt.Printf("%q", dump)

		// We only handle GET & HEAD requests for now.
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		uri := r.RequestURI
		// fmt.Println("RequestURI: " + uri)

		// Try to get from cache.
		mime, cr, err := cache.Get(uri)
		if err == nil {
			// Cache hit.
			w.Header().Set("X-Cache", "HIT")
			// log.Println("cache hit")

			defer cr.Close()
			w.Header().Set("Content-Type", mime)
			switch r.Method {
			case "GET":
				// w.WriteHeader(200) // TODO I'm pretty sure 200 is the default?
				// TODO Would be good to set Content-Length header - but we don't know it until after the Copy - hide the reader inside Cache.Get?
				length, err := io.Copy(w, cr)
				if err != nil {
					log.Printf("io.Copy error during GET: %v\n", err)
					// TODO Does this error work ok here? We may have sent content / set content type. To fix this, we should flip-flop through a buffer.
					http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
					return
				}
				log.Printf("decompressed content size %s", byteCountDecimal(length))
			case "HEAD":
				// w.WriteHeader(200) // TODO I'm pretty sure 200 is the default?
				length, err := io.Copy(ioutil.Discard, cr)
				if err != nil {
					log.Printf("io.Copy error during HEAD: %v\n", err)
					http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Length", strconv.Itoa(int(length)))
				log.Printf("decompressed content size %s (HEAD)", byteCountDecimal(length))
			}
			return
		}
		if err != ErrCacheMiss {
			log.Printf("cache.Get error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}

		// Cache miss - fetch and cache.
		w.Header().Set("X-Cache", "MISS")
		// log.Println("cache miss")

		// TODO Consider rate limiting - per destination host.
		// See also https://github.com/internetarchive/heritrix3/wiki/Politeness-parameters
		// Currently I'm considering doing this in the client, so it can act differently for hits/misses.

		// Build the request.
		// req, err := http.NewRequest("GET", uri, nil)
		req, err := retryablehttp.NewRequest("GET", uri, nil)
		if err != nil {
			log.Printf("http.NewRequest error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		copyHeaders(req.Header, r.Header)

		// Make the request.
		client, err := newClient()
		if err != nil {
			log.Printf("newClient error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		rstart := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("client.Do error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		rdur := time.Since(rstart)
		log.Printf("upstream request duration %v", rdur)
		responseTime := float64(rdur) / float64(time.Millisecond)

		defer resp.Body.Close()

		// TODO Should we check content type is text/HTML/JSON/CSS (not binary data) ?

		// Read the response body, limiting the max size.
		lr := io.LimitedReader{R: resp.Body, N: maxBodySize + 1}
		body, err := ioutil.ReadAll(&lr)
		if err != nil {
			log.Printf("ioutil.ReadAll error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		if lr.N == 0 {
			// Exceeded max body size.
			log.Println("response too big error")
			m := fmt.Sprintf("Body exceeds maximum size (%s)", byteCountDecimal(maxBodySize))
			http.Error(w, m, http.StatusInternalServerError)
			return
		}

		// Check status code is good - we only accept 200 ok.
		if resp.StatusCode != 200 {
			// Upstream error.
			log.Printf("upstream error: non-200 status code (%d)\n", resp.StatusCode)
			// TODO We could return the original status code + body? No...
			// TODO We should probably return a 500 here - we only handle 200.
			http.Error(w, resp.Status, resp.StatusCode)
			return
		}

		// TODO Check page body against reject rules.

		mime = resp.Header.Get("Content-Type")
		etag := resp.Header.Get("ETag")
		lastMod := resp.Header.Get("Last-Modified")

		// Put it in the cache.
		err = cache.Put(uri, mime, etag, lastMod, body, responseTime)
		if err != nil {
			log.Printf("cache.Put error: %v\n", err)
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}
		// log.Printf("cached content size %s", byteCountDecimal(int64(len(body))))

		// Finally, send to client.
		w.Header().Set("Content-Type", mime)
		switch r.Method {
		case "GET":
			// w.WriteHeader(200) // TODO I'm pretty sure 200 is the default?
			w.Write(body)
		case "HEAD":
			// w.WriteHeader(200) // TODO I'm pretty sure 200 is the default?
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		}
	}
}

func newClient() (*retryablehttp.Client, error) {
	// TODO Client strategy - shared per domain? new per request?
	// TODO Client configuration - see https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779

	// TODO Note that because we use a retrying client, this means outgoing HTTP requests can now take a longer time.
	// Do we need to make the HTTP server and the requesting client have longer timeouts to handle this? Review this.

	// client := &http.Client{}
	client := retryablehttp.NewClient()
	// client.Logger = nil
	return client, nil
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
				//				log.Printf("Copying header: %s = %s\n", k, v)
			}
		}
	}
}

func shouldCopyHeader(headerKey string) bool {
	switch http.CanonicalHeaderKey(headerKey) {
	case "Accept-Encoding":
		// http.Client handles this itself.
		// If we copy it across, and it says gzip (it will),
		// then we have to manually handle gzip decoding.
		return false
	}
	return true
}
