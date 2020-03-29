# progszy

[![BSD3](https://img.shields.io/badge/license-BSD3-blue.svg?style=flat)](LICENSE.md)
[![Build Status](https://img.shields.io/travis/jimsmart/progszy/master.svg?style=flat)](https://travis-ci.org/jimsmart/progszy)
[![codecov](https://codecov.io/gh/jimsmart/progszy/branch/master/graph/badge.svg)](https://codecov.io/gh/jimsmart/progszy)
[![Go Report Card](https://goreportcard.com/badge/github.com/jimsmart/progszy)](https://goreportcard.com/report/github.com/jimsmart/progszy)
[![Godoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/jimsmart/progszy)

progszy is a hard-caching HTTP(S) proxy server (with programmatic cache management), designed for use as part of a data-scraping pipeline.

It is both a standalone executable CLI program, and a Go package.

It is **not** suitable for use as a regular HTTP(S) caching proxy for humans surfing with web browsers.

progszy should work with any HTTP client, but currently has only been tested with Go's http.Client.

## Caching

Cached content is persisted in an [SQLite](https://www.sqlite.org) database, using [Zstandard](https://www.zstd.net) compression, enabling cached content to be retrieved [faster](https://www.sqlite.org/fasterthanfs.html) than regular file system reads, while also providing convenient packaging of cached content and saving storage space.

A separate single-file database is created per domain, to cache its respective content (that is: content is 'binned' according to the domain's base/root name). Database filenames also contain a creation timestamp. 

For example, request responses for `http://www.example.com/index.html` and `http://foo.bar.example.com/index.html` will both get cached in the same database, having a filename like `example.com-2020-03-20-1640.sqlite`.

We may review/change this binning/naming strategy at a later date.

### Caching Strategy

progszy intentionally makes no use of HTTP headers relating to cached content control that are normally utilised by browsers and other caching proxies.

The body content and appropriate headers for all `200 Ok` responses are hard-cached — unless the body matches a given filter (see `X-Cache-Reject`, below).

Content exceeding an arbitrary maximum body size of 64mb is not cached nor proxied, and instead returns a `412 Precondition Failed` response to the client. We may review this decision/behaviour at a later date.

Cache eviction/management is manual-only at present. Later we will add a REST API for programmatic cache management.

## HTTP(S) Proxy

The CLI version of progszy operates as a standalone HTTP(S) proxy server. By default it listens on port 5595, for which the client's proxy configuration URL would be `http://127.0.0.1:5595`.

TODO 127.0.0.1 or 0.0.0.0 or localhost ?

Incoming requests can be either vanilla HTTP, or can be HTTPS (using `CONNECT` protocol). 

When proxying HTTPS requests, the connection is intercepted by a man-in-the-middle (MITM) hijack, to allow both caching and the application of rules, and the resulting outbound stream is then re-encrypted using a private certificate, before being passed to the client. Note that clients wishing to proxy HTTPS requests using progszy will need specific configuration to prevent/ignore the resulting certificate mismatch errors caused by this process. See tests for an example of how this is done in Go.

Outgoing HTTP requests utilise automatic retries with exponential backoff. Internal HTTP clients use a shared transport with pooling. Connections are not explicitly rate-limited.

Currently, progszy only supports HTTP `GET`, `HEAD` and `CONNECT` methods. Note that support for the `HEAD` method is not actually particularly useful in this context, and really only exists for spec compliance.

### HTTP Headers

progszy makes use of custom HTTP `X-*` headers to both control features and report status to the client.

#### Request Headers
 
 - `X-Cache-Reject` headers control early rejection/filtering of incoming content. Each header value is compiled into a regexp reject rule: if the content body matches any filter, the request response is not cached, and instead a `412 Precondition Failed` is returned to the client. See tests for example usage. Note that cache hits (requests for already cached content) are not currently affected by the use of this header.
 - `X-Cache-SSL: INSECURE` forces use of an internal HTTP client configured to skip SSL certificate validation during the upstream/outbound request. See tests for example usage.

Incoming `X-*` headers are not copied to outgoing requests.

#### Response Headers

 - `X-Cache` value will be `HIT` or `MISS` accordingly.
 - `Content-Type`, `Content-Language`, `ETag` and `Last-Modified` headers on incoming responses all have their value persisted to the cache, and restored appropriately on outgoing responses to the client.

## Installation

### Binary Executable

TODO 

TODO Document any runtime dependencies - I believe the binary should be standalone?

### Build From Source

TODO 

TODO Document build-time dependencies that require preinstallation: Sqlite? gcc toolchain to build zstd?

## Usage Examples

Once built/installed, progszy can be invoked via the command line, as follows...

Get help / usage instructions:

```bash
$ ./progszy --help
Usage of ./progszy:
  -cache string
    	Cache location (default "./cache")
  -port int
    	Port number to listen on (default 5595)
```

Run progszy with default settings:

```bash
$ ./progszy
Cache location /<path-to-current-folder>/cache
Listening on port 5595
```

Run using custom configuration:

```bash
$ ./progszy -port 8080 -cache /foo/bar/store
Cache location /foo/bar/store
Listening on port 8080
```

Press <kbd>control</kbd>+<kbd>c</kbd> to halt execution — progszy will attempt to cleanly complete any in-flight connections before exiting.

## Package Documentation

GoDocs [https://godoc.org/github.com/jimsmart/progszy](https://godoc.org/github.com/jimsmart/progszy)

## Testing

To run the tests execute `go test` inside the project folder.

For a full coverage report, try:

```bash
$ go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## Project Dependencies

Packages used by progszy (and their licensing):

- SQLite driver [https://github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) (MIT license)
    - SQLite database [https://www.sqlite.org/](https://www.sqlite.org) (Public Domain, explicit)
- Zstd wrapper [https://github.com/DataDog/zstd](https://github.com/DataDog/zstd) (Simplified BSD 3-Clause license)
    - Zstandard [https://github.com/facebook/zstd](https://github.com/facebook/zstd) (BSD and GPL 2.0, dual licensed)
- goproxy [https://github.com/elazarl/goproxy](https://github.com/elazarl/goproxy) (BSD 3-Clause license)
- retryablehttp [https://github.com/hashicorp/go-retryablehttp](https://github.com/hashicorp/go-retryablehttp) (MPL 2.0 license)
- cleanhttp [https://github.com/hashicorp/go-cleanhttp](https://github.com/hashicorp/go-cleanhttp) (MPL 2.0 license)
- publicsuffix [https://github.com/weppos/publicsuffix-go](https://github.com/weppos/publicsuffix-go) (MIT license)
- Go standard library. (BSD-style license)
- [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) are used in the tests. (MIT license)

— Many thanks to the authors and contributors of these packages.

## License

progszy is copyright 2020 by Jim Smart and released under the [BSD 3-Clause License](LICENSE.md)

## History

TODO

- v0.0.1: Initial release.
- 2020-01-01: Work in progress.