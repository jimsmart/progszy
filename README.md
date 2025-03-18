# Progszy

[![BSD3](https://img.shields.io/badge/license-BSD3-blue.svg?style=flat)](LICENSE.md)
[![Build Status](https://github.com/jimsmart/progszy/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/jimsmart/progszy/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jimsmart/progszy)](https://goreportcard.com/report/github.com/jimsmart/progszy)
[![Go Reference](https://img.shields.io/badge/-reference-277d9c?&logo=go&logoColor=fafafa&labelColor=5c5c5c)](https://pkg.go.dev/github.com/jimsmart/progszy)
<!-- [![codecov](https://codecov.io/gh/jimsmart/progszy/branch/master/graph/badge.svg)](https://codecov.io/gh/jimsmart/progszy) -->

Progszy is a hard-caching HTTP(S) proxy server (with programmatic cache management), designed for use as part of a data-scraping pipeline.

- Brings stable reproducability to web data-scraping pipelines.
- Improves web scraper development workflow, via fast controlled caching of HTTP responses.
- Improves debugging of failed live scrapes, download the cached data for consistent reproducible local re-runs.
- Improves scraper performance and reduces remote server load, when resuming partial/incomplete scrapes.
- Fast and compact data caching: SQLite database storage layer, Zstandard compression used for HTTP body.

It is both a standalone executable CLI program, and a Go package.

It is **not** suitable for use as a regular HTTP(S) caching proxy for humans surfing with web browsers.

Progszy should work with any HTTP client, but currently has only been tested with Go's http.Client.

## Caching

Cached content is persisted in an [SQLite](https://www.sqlite.org) database, using [Zstandard](https://www.zstd.net) compression, enabling cached content to be retrieved [faster](https://www.sqlite.org/fasterthanfs.html) than regular file system reads, while also providing convenient packaging of cached content and saving storage space.

A separate single-file database is created per domain, to cache its respective content (that is: content is 'binned' according to the root domain name). Database filenames also contain a creation timestamp.

For example, request responses for `http://www.example.com/index.html` and `http://foo.bar.example.com/index.html` will both get cached in the same database, having a filename like `example.com-2020-03-20-1640.sqlite`.

We may review/change this binning/naming strategy at a later date.

### Caching Strategy

Progszy *intentionally* makes **no** use of HTTP headers relating to cached content control that are normally utilised by browsers and other caching proxies.

The body content and appropriate headers for all `200 Ok` responses are hard-cached — unless the body matches a given filter (see `X-Cache-Reject`, below).

Content exceeding an arbitrary maximum body size of 512mb is not cached nor proxied, and instead returns a `507 Insufficient Storage` response to the client. We may review this decision/behaviour at a later date.

Cache eviction/management is manual-only at present. Later we will add a REST API for programmatic cache management.

## HTTP(S) Proxy

The CLI version of Progszy operates as a standalone HTTP(S) proxy server. By default it listens on port 5595, for which the client's proxy configuration URL would be `http://127.0.0.1:5595`. It should be noted that currently Progszy binds only to IP 127.0.0.1, which is not suitable for access from a remote IP (without the use of an SSH tunnel).

Incoming requests can be either vanilla HTTP, or can be HTTPS (using `CONNECT` protocol).

When proxying HTTPS requests, the connection is intercepted by a man-in-the-middle (MITM) hijack, to allow both caching and the application of rules, and the resulting outbound stream is then re-encrypted using a private certificate, before being passed to the client. Note that clients wishing to proxy HTTPS requests using Progszy will need specific configuration to prevent/ignore the resulting certificate mismatch errors caused by this process. See tests for an example of how this is done in Go.

Outgoing HTTP requests utilise automatic retries with exponential backoff. Internal HTTP clients use a shared transport with pooling, and support upstream proxy chaining. Connections are not explicitly rate-limited.

Currently, Progszy only supports HTTP `GET`, `HEAD` and `CONNECT` methods. Note that support for the `HEAD` method is not actually particularly useful in this context, and really only exists for spec compliance.

### HTTP Headers

Progszy makes use of custom HTTP `X-*` headers to both control features and report status to the client.

#### Request Headers

- `X-Cache-Reject` headers control early rejection/filtering of incoming content. Each header value is compiled into a regexp reject rule: if the content body matches any filter, the request response is not cached, and instead a `412 Precondition Failed` is returned to the client. See tests for example usage. Note that cache hits (requests for already cached content) are not currently affected by the use of this header.
- `X-Cache-SSL: INSECURE` forces use of an internal HTTP client configured to skip SSL certificate validation during the upstream/outbound request. See tests for example usage.
- `X-Cache-Flush: TRUE` forces the creation of a new cache database bin for the requested URL.

Incoming `X-*` headers are not copied to outgoing requests.

#### Response Headers

- `X-Cache` value will be `HIT`, `MISS` or `FLUSHED` accordingly. For cache hits and misses, the following headers are also present:
- `X-Cache-Timestamp` indicates when the content was originally cached (RFC3339 format with nanosecond precision).
- `Content-Length` value is set accordingly.
- `Content-Type`, `Content-Language`, `ETag` and `Last-Modified` headers from incoming responses all have their value persisted to the cache, and restored appropriately on outgoing responses to the client.

## Installation

### Binary Executable

Pre-built binary executables for Linux and Windows are available for download from
the [latest release](https://github.com/jimsmart/progszy/releases/latest) page.

### Build From Source

First, ensure you have a working Go environment. See [Go 'Getting Started' documentation](https://golang.org/doc/install).

Then fetch the code, build and install the binary:

```text
go get github.com/jimsmart/progszy/cmd/progszy
```

By default, the resulting binary executable will be `~/go/bin/progszy` (assuming no customisation has been made to `$GOPATH` or `$GOBIN`).

## Usage Examples

Once built/installed, Progszy can be invoked via the command line, as follows...

Get help / usage instructions:

```text
$ ./progszy --help
Usage of ./progszy:
  -cache string
        Cache location (default "./cache")
  -port int
        Port number to listen on (default 5595)
  -proxy string
        Upstream HTTP(S) proxy URL (e.g. "http://10.0.0.1:8080")
```

Run Progszy with default settings:

```text
$ ./progszy
Cache location /<path-to-current-folder>/cache
Listening on port 5595
```

Run using custom configuration:

```text
$ ./progszy -port=8080 -cache=/foo/bar/store -proxy=http://10.10.0.1:9000
Cache location /foo/bar/store
Upstream proxy http://10.10.0.1:9000
Listening on port 8080
```

Press <kbd>control</kbd>+<kbd>c</kbd> to halt execution — Progszy will attempt to cleanly complete any in-flight connections before exiting.

## Developer Information

### Package Documentation

GoDocs [https://pkg.go.dev/github.com/jimsmart/progszy](https://pkg.go.dev/github.com/jimsmart/progszy)

#### Local GoDocs

Change folder to project root, and run:

```text
godoc -http=:6060 -notes="BUG|TODO"
```

Open a web browser and navigate to [http://127.0.0.1:6060/pkg/github.com/jimsmart/progszy/](http://127.0.0.1:6060/pkg/github.com/jimsmart/progszy/)

### Testing

To run the tests execute `go test` inside the project root folder.

For a full coverage report, try:

```text
go test -coverprofile=coverage.out && go tool cover -html=coverage.out
```

### GitHub Build Automation

This repo uses the following GitHub Action workflow automations:

#### Github Actions

Documentation [https://docs.github.com/en/actions](https://docs.github.com/en/actions)

- `.github/workflows/build.yml` - Automatically runs on all push actions to this repo. Builds project, runs go vet & golint, runs tests, reports coverage (coverage reporting is currently disabled, due to this repo currently being private).
- `.github/workflows/dummy-release.yml` - Manually run pre-release workflow. Runs the same actions as the 'release' action (below), but skips publishing. Use this as a dry run, before pushing a version-tagged commit to the repo to trigger publication of a release.
- `.github/workflows/release.yml` - Automatically runs on all push actions to this repo that specify a tag of format `"v*.*.**"`. Installs cross-compilers, runs GoReleaser to  build all target binaries, package tars/zips, and create a draft release using the resulting assets. Publication of this release must then be manually confirmed on GitHub (by choosing to edit the release, and pressing the green 'Publish release' button).

#### GoReleaser

Website [https://goreleaser.com/](https://goreleaser.com/)

`.goreleaser.yml` contains GoReleaser configuration for release builds, handling cross-compilation, packaging and creation of a (draft) release on GitHub. It is invoked by the above mentioned GitHub Actions, 'release' and 'dummy release'.

### Release Publication

#### 1. Dry Run of Release Build Workflow

First, go to this repo's [Actions page](https://github.com/jimsmart/progszy/actions), and manually run the 'dummy release' action workflow, addressing any issues that may arise.

#### 2. Tag Version & Push

Once the 'dummy release' action workflow completes ok, then make a version-tagged push to the repo, using a command similar to:

```text
git tag v0.0.1 && git push origin v0.0.1
```

(Amending the version number accordingly)

On completion of the push, the 'release' action workflow will automatically begin execution. Wait for it to complete.

#### 3. Confirm Publication

GoRelease is configured here to only publish draft releases.

On successful completion of the 'release' workflow execution, go to the repo's [releases page](https://github.com/jimsmart/progszy/releases), find the new draft release, edit it (by clicking the pencil icon), check all is well, then click the green 'Publish release' button.

### Project Dependencies

Packages used by Progszy (and their licensing):

- SQLite driver [https://github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) (MIT license)
  - SQLite database [https://www.sqlite.org/](https://www.sqlite.org) (Public Domain, explicit)
- Zstd wrapper [https://github.com/valyala/gozstd](https://github.com/valyala/gozstd) (MIT license)
  - Zstandard [https://github.com/facebook/zstd](https://github.com/facebook/zstd) (BSD and GPL 2.0, dual licensed)
- goproxy [https://github.com/elazarl/goproxy](https://github.com/elazarl/goproxy) (BSD 3-Clause license)
- retryablehttp [https://github.com/hashicorp/go-retryablehttp](https://github.com/hashicorp/go-retryablehttp) (MPL 2.0 license)
- cleanhttp [https://github.com/hashicorp/go-cleanhttp](https://github.com/hashicorp/go-cleanhttp) (MPL 2.0 license)
- publicsuffix [https://github.com/weppos/publicsuffix-go](https://github.com/weppos/publicsuffix-go) (MIT license)
- Go standard library. (BSD-style license)
- [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) are used in the tests. (MIT license)

— Many thanks to the authors and contributors of these packages.

## License

Progszy is copyright 2020–2025 by Jim Smart and released under the [BSD 3-Clause License](LICENSE.md).

## History

- v0.0.16 (2025-03-18) Updated dependencies.
- v0.0.15 (2022-12-16) Updated dependencies.
- v0.0.14 (2022-02-03) Improved documentation.
- v0.0.12 (2022-02-03) Increase maxBodySize to 512mb. Cleanups.
- v0.0.11 (2022-01-27) Test fixup. Updated dependencies.
- v0.0.10 (2021-06-21) Require Go 1.15 instead of 1.16.
- v0.0.9 (2021-06-21) Updated dependencies.
- v0.0.3 (2021-04-21) Automated releases.
- v0.0.1 (2021-04-20) Work in progress. Initial test release.
