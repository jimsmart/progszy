package progszy_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jimsmart/progszy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Progszy", func() {

	// Hook into some kind of Before method.
	// Start proxy.
	// HttpClient configured to use a proxy.
	// Request a URL.

	// It("should run", func() {
	// 	progszy.Run()
	// 	Expect(1).To(Equal(1))
	// })

	var server *httptest.Server
	var cache progszy.Cache

	BeforeEach(func() {
		// cache = progszy.NewMemCache()
		cache = progszy.NewSqliteCache(testCachePath)
		server = httptest.NewServer(http.HandlerFunc(progszy.ProxyHandlerWith(cache)))
		// fmt.Println("server started")
	})

	AfterEach(func() {
		server.Close()
		// fmt.Println("server closed")
		err := cache.CloseAll()
		Expect(err).To(BeNil())

		// TODO(js) This is somewhat clunky.
		err = deleteSqliteDBs()
		Expect(err).To(BeNil())
	})

	It("should proxy requests", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(server.URL)
		Expect(err).To(BeNil())

		resp, err := c.Get("http://books.toscrape.com")
		Expect(err).To(BeNil())

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).To(BeNil())
		Expect(body).To(ContainSubstring("Books"))

		ch := resp.Header.Get("X-Cache")
		Expect(ch).To(Equal("MISS"))

		c.CloseIdleConnections()
	})

	It("should proxy requests using the cache", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(server.URL)
		Expect(err).To(BeNil())

		resp1, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		body1, err := ioutil.ReadAll(resp1.Body)
		Expect(err).To(BeNil())
		Expect(body1).To(ContainSubstring("Books"))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		resp2, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		body2, err := ioutil.ReadAll(resp2.Body)
		Expect(err).To(BeNil())
		// Expect(body2).To(ContainSubstring("Books"))
		Expect(body2).To(Equal(body1))

		ch2 := resp2.Header.Get("X-Cache")
		Expect(ch2).To(Equal("HIT"))

		c.CloseIdleConnections()
	})

	It("should correctly handle HEAD method after a GET", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(server.URL)
		Expect(err).To(BeNil())

		resp1, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		body1, err := ioutil.ReadAll(resp1.Body)
		Expect(err).To(BeNil())
		Expect(body1).To(ContainSubstring("Books"))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		resp2, err := c.Head("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		length, err := strconv.Atoi(resp2.Header.Get("Content-Length"))
		Expect(err).To(BeNil())
		Expect(length).To(Equal(len(body1)))

		ch2 := resp2.Header.Get("X-Cache")
		Expect(ch2).To(Equal("HIT"))

		c.CloseIdleConnections()
	})

	It("should correctly handle HEAD method", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(server.URL)
		Expect(err).To(BeNil())

		resp1, err := c.Head("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		length, err := strconv.Atoi(resp1.Header.Get("Content-Length"))
		Expect(err).To(BeNil())
		Expect(length).ToNot(BeZero())

		c.CloseIdleConnections()
	})

})

func newProxyClient(proxyURL string) (*http.Client, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
	return client, nil
}

func deleteSqliteDBs() error {

	// return nil

	// We will delete files with the correct extension.
	var files []string
	filterFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".sqlite" { // TODO extensions
			return nil
		}
		files = append(files, path)
		return nil
	}

	// log.Printf("cleaning up in %s\n", testCachePath)
	err := filepath.Walk(testCachePath, filterFn)
	if err != nil {
		return err
	}

	for _, file := range files {
		// log.Printf("deleting %s", file)
		err = os.Remove(file)
		if err != nil {
			log.Printf("error deleting %s - %v", file, err)
		}
	}
	return nil
}
