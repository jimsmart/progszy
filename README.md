# progszy

progszy is a hard-caching HTTP(S) proxy server, designed for use as part of a data-scraping pipeline.

It is both a standalone CLI tool, and a Go package.

It is **not** suitable for use as a regular HTTP(S) caching proxy, e.g. with web browsers.

progszy should work with any client, but has currently only been tested with Go's http.Client.

## Caching

Cached content is persisted in an [SQLite](https://www.sqlite.org/index.html) database, using [Zstandard](https://github.com/facebook/zstd) compression, enabling cached content to be retrieved [faster](https://www.sqlite.org/fasterthanfs.html) than regular file system reads, while also providing convenient packaging of cached content and saving storage space.

A separate database is created for the content of each 'public domain', and database filenames also contain a creation timestamp.

### Caching Strategy

progszy intentionally makes no use of HTTP headers relating to cached content control that are normally utilised by browsers and other caching proxies.

The content for all `200 Ok` responses is hard-cached (unless it matches a given filter, see below).

Content exceeding an arbitrary maximum body size of 64mb is not cached, and instead returns a `412 Precondition Failed`. We may review this decision/behaviour at a later date.

Cache eviction/management is manual-only at present. Later we will add a REST API for programmatic cache management.

## HTTP(S) Proxy

As a standalone server, it serves as a standard HTTP proxy server, on port 5995 by default, for which the client configuration URL would be `http://127.0.0.1:5595`.

TODO 127.0.0.1 or 0.0.0.0 or localhost ?

Incoming requests can be either vanilla HTTP, or can be HTTPS. When proxying HTTPS requests, the connection is man-in-the-middled (MITM) to allow caching, application of rules, etc., and the resulting outbound stream is then re-encrypted using a private certificate before being passed to the client. (Note that clients wishing to proxy HTTPS requests will need configuration to prevent certificate mismatch errors.)

Outgoing HTTP requests utilise automatic retries with exponential backoff. Internal HTTP clients use a shared transport with pooling.

### HTTP Headers

progszy makes use of HTTP `X-*` headers to both control features and report status. Incoming `X-*` headers are not copied to outgoing requests.

#### Request Headers
 
 - `X-Cache-SSL: INSECURE` forces use of a client that skips SSL certificate validation for upstream requests.
 - `X-Cache-Reject` headers control early rejection/filtering of content. Each value is compiled into a regexp reject rule. If the content body matches any filter, the request is not cached, and instead a `500` is returned. See tests for examples.

#### Response Headers

 - `X-Cache` value will be `HIT` or `MISS`
 - TODO document regular headers

## Installation

TODO installation instructions

### Dependecies

- SQlite driver https://github.com/mattn/go-sqlite3
- Zstd wrapper https://github.com/DataDog/zstd
- goproxy https://github.com/elazarl/goproxy
- retryablehttp https://github.com/hashicorp/go-retryablehttp
- cleanhttp https://github.com/hashicorp/go-cleanhttp
- publicsuffix https://github.com/weppos/publicsuffix-go/publicsuffix
- Standard library.
- [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) are used in the tests.

## Example / Usage

TODO example

## Documentation

GoDocs [https://godoc.org/github.com/jimsmart/progszy](https://godoc.org/github.com/jimsmart/progszy)

## Testing

To run the tests execute `go test` inside the project folder.

For a full coverage report, try:

```bash
$ go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## License

TODO license BSD? What do deps use?

## History

- v0.0.1: Initial release.
