package progszy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/valyala/gozstd"
	"github.com/weppos/publicsuffix-go/publicsuffix"
)

type Cache interface {
	Get(uri string) (*CacheRecord, error)
	Put(cr *CacheRecord) error
	CloseAll() error
	Flush(uri string) error
}

// TODO Add Head method, using cached info.

// TODO Switch to using cacheRecord as inputs/outputs of Cache methods.

// TODO Refactor Cache interface, accounting for the above.

// ErrCacheMiss occurs when a given URL is not in the cache.
var ErrCacheMiss = errors.New("progszy: cache miss")

type CacheRecord struct {
	// Key is the normalised URL.
	Key string
	// URL is the originally requested URL.
	URL string
	// BaseDomain is the friendly domain name.
	BaseDomain string
	// Status code of response.
	Status int
	// Protocol originally used for response.
	Protocol string
	// ContentLanguage value (or empty string).
	ContentLanguage string
	// ContentType is the MIME type.
	ContentType string
	// ETag value (or empty string).
	ETag string
	// LastModified value (or empty string).
	LastModified string
	// ZstdBody holds the Zstd compressed HTTP body.
	ZstdBody []byte
	// CompressedLength is the length of ZstdBody.
	CompressedLength int64
	// ContentLength is the original content length.
	ContentLength int64
	// ResponseTime is the duration of the original request, in ms.
	ResponseTime float64
	// MD5 is the md5 sum of the uncompressed body.
	MD5 string
	// Created is the time this record was created.
	Created time.Time
}

func (r *CacheRecord) Body() (io.ReadCloser, error) {
	body, err := gozstd.Decompress(nil, r.ZstdBody)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(body)), nil
}

const logCompressionStats = false

func (r *CacheRecord) SetBody(body []byte) error {
	olen := int64(len(body))
	start := time.Now()

	// cbody := gozstd.Compress(nil, body) // Default compression level.
	cbody := gozstd.CompressLevel(nil, body, 20)

	if logCompressionStats {
		clen := int64(len(cbody))
		ratio := float64(clen) / float64(olen)
		dur := time.Since(start)
		log.Printf("compressed %s to %s (ratio %.2f), duration %v", byteCountDecimal(olen), byteCountDecimal(clen), ratio, dur)
	}

	h := md5.New()
	h.Write(body)
	r.MD5 = hex.EncodeToString(h.Sum(nil))
	// log.Printf("md5 %s", r.MD5)

	r.CompressedLength = int64(len(cbody))
	r.ContentLength = int64(len(body))

	r.ZstdBody = cbody
	return nil
}

// TODO cacheRecord should hold ETag, LastModified, Content-Length(?), md5(?)

func NewCacheRecord(uri string, status int, proto, lang, mime, etag, lastMod string, body []byte, responseTime float64, created time.Time) (*CacheRecord, error) {

	nurl, bd, err := cacheRecordKey(uri)
	if err != nil {
		return nil, err
	}

	r := &CacheRecord{
		Key:             nurl,
		URL:             uri,
		BaseDomain:      bd,
		Status:          status,
		Protocol:        proto,
		ContentLanguage: lang,
		ContentType:     mime,
		ETag:            etag,
		LastModified:    lastMod,
		ResponseTime:    responseTime,
		Created:         created.UTC(),
	}

	err = r.SetBody(body)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func cacheRecordKey(uri string) (string, string, error) {
	// Normalise url.
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	NormalisePath(u)
	err = NormaliseQuery(u)
	if err != nil {
		return "", "", err
	}
	h, err := BaseDomainName(u)
	if err != nil {
		return "", "", err
	}

	return u.String(), h, nil
}

func NormaliseQuery(u *url.URL) error {
	if len(u.RawQuery) == 0 {
		return nil
	}

	v, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		// TODO If there is a problem with the query string, should we just warn?
		return err
	}
	u.RawQuery = v.Encode()
	return nil
}

func NormalisePath(u *url.URL) {
	hasSlash := strings.HasSuffix(u.Path, "/")

	// clean up path, removing duplicate `/`
	u.Path = path.Clean(u.Path)
	u.RawPath = path.Clean(u.RawPath)

	if hasSlash && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		u.RawPath += "/"
	}
}

func BaseDomainName(u *url.URL) (string, error) {
	h := u.Host

	// Strip port number, if present.
	if i := strings.IndexByte(h, ':'); i != -1 {
		h = h[:i]
	}

	// Is it an IP number?
	if net.ParseIP(h) != nil {
		return h, nil
	}
	// Is it localhost?
	if h == "localhost" {
		// TODO Could keep port numbers for localhost?
		return h, nil
	}
	// TODO What about local names? e.g. myserver or myserver.lan
	// TODO We currently ignore port numbers. Is that ok? Would it be better not to? Perhaps.
	return publicsuffix.Domain(h)
}
