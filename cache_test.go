package progszy_test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jimsmart/progszy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testCachePath string

func init() {
	// Location of cache for testing = wd + "test".
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	testCachePath = filepath.Join(path, "test")
}

var _ = Describe("Cache", func() {

	Describe("Cache helpers", func() {

		It("should normalise url query strings", func() {
			u, _ := url.Parse("http://10.0.0.1/abc?z=1&y=2&y=3&x")
			err := progszy.NormaliseQuery(u)
			Expect(err).To(BeNil())
			Expect(u.String()).To(Equal("http://10.0.0.1/abc?x=&y=2&y=3&z=1"))
		})

		It("should normalise url paths", func() {
			u, _ := url.Parse("http://10.0.0.1/a//bc/")
			progszy.NormalisePath(u)
			Expect(u.String()).To(Equal("http://10.0.0.1/a/bc/"))
		})

		It("should return the host for localhost", func() {
			u, _ := url.Parse("http://localhost/a/b/c")
			d, err := progszy.BaseDomainName(u)
			Expect(err).To(BeNil())
			Expect(d).To(Equal("localhost"))
		})

		It("should return the host for localhost:80", func() {
			u, _ := url.Parse("http://localhost:80")
			d, err := progszy.BaseDomainName(u)
			Expect(err).To(BeNil())
			Expect(d).To(Equal("localhost"))
		})

		It("should return the host for an IPv4", func() {
			u, _ := url.Parse("http://10.0.0.1/a/b/c")
			d, err := progszy.BaseDomainName(u)
			Expect(err).To(BeNil())
			Expect(d).To(Equal("10.0.0.1"))
		})

		It("should return the host for www.example.co.uk", func() {
			u, _ := url.Parse("http://www.example.co.uk/")
			d, err := progszy.BaseDomainName(u)
			Expect(err).To(BeNil())
			Expect(d).To(Equal("example.co.uk"))
		})

		It("should return the host for foo.www.example.co.uk", func() {
			u, _ := url.Parse("http://foo.www.example.co.uk/")
			d, err := progszy.BaseDomainName(u)
			Expect(err).To(BeNil())
			Expect(d).To(Equal("example.co.uk"))
		})

	})

	// Describe("MemCache methods", func() {

	// 	It("should return a 'miss' error for an unknown URL", func() {
	// 		c := progszy.NewMemCache()
	// 		_, _, err := c.Get("http://example.com/")
	// 		Expect(err).To(Equal(progszy.ErrCacheMiss))
	// 		err = c.CloseAll()
	// 		Expect(err).To(BeNil())
	// 	})

	// 	It("should put things into the cache and get them out again", func() {

	// 		content := []byte("fake-content")

	// 		c := progszy.NewMemCache()
	// 		err := c.Put("http://example.com/", "text/html", "", "", content, 0)
	// 		Expect(err).To(BeNil())
	// 		m, r, err := c.Get("http://example.com/")
	// 		Expect(err).To(BeNil())
	// 		Expect(m).To(Equal("text/html"))
	// 		defer r.Close()
	// 		b, err := ioutil.ReadAll(r)
	// 		Expect(err).To(BeNil())
	// 		Expect(b).To(Equal(content))
	// 		err = c.CloseAll()
	// 		Expect(err).To(BeNil())
	// 	})

	// })

	Describe("SqliteCache methods", func() {

		AfterEach(func() {
			// TODO(js) This is somewhat clunky.
			err := deleteSqliteDBs()
			Expect(err).To(BeNil())
		})

		It("should return a 'miss' error for an unknown URL", func() {
			c := progszy.NewSqliteCache(testCachePath)
			_, err := c.Get("http://example.com/")
			Expect(err).To(Equal(progszy.ErrCacheMiss))
			err = c.CloseAll()
			Expect(err).To(BeNil())
		})

		It("should put things into the cache and get them out again", func() {

			content := []byte("fake-content")

			c := progszy.NewSqliteCache(testCachePath)
			cr, err := progszy.NewCacheRecord("http://example.com/", 200, "", "", "text/html", "", "", content, 0, time.Now())
			Expect(err).To(BeNil())
			err = c.Put(cr)
			Expect(err).To(BeNil())
			cr, err = c.Get("http://example.com/")
			Expect(err).To(BeNil())
			Expect(cr.ContentType).To(Equal("text/html"))
			r, err := cr.Body()
			Expect(err).To(BeNil())
			defer r.Close()
			b, err := ioutil.ReadAll(r)
			Expect(err).To(BeNil())
			Expect(b).To(Equal(content))
			err = c.CloseAll()
			Expect(err).To(BeNil())
		})

	})

})
