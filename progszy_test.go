package progszy_test

import (
	"crypto/tls"
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
	var proxyURL string

	BeforeEach(func() {
		// cache = progszy.NewMemCache()
		cache = progszy.NewSqliteCache(testCachePath)
		// server = httptest.NewServer(http.HandlerFunc(progszy.ProxyHandlerWith(cache)))
		server = httptest.NewServer(progszy.ProxyHandlerWith(cache))
		// fmt.Println("server started")
		// proxyURL = "https" + server.URL[4:]
		proxyURL = server.URL
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

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		resp, err := c.Get("http://books.toscrape.com")
		Expect(err).To(BeNil())

		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).To(BeNil())
		Expect(body).To(ContainSubstring("Books"))

		ch := resp.Header.Get("X-Cache")
		Expect(ch).To(Equal("MISS"))

		c.CloseIdleConnections()
	})

	It("should proxy HTTPS requests", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		resp, err := c.Get("https://webscraper.io/test-sites/e-commerce/allinone")
		Expect(err).To(BeNil())

		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).To(BeNil())
		Expect(body).To(ContainSubstring("Computers"))

		ch := resp.Header.Get("X-Cache")
		Expect(ch).To(Equal("MISS"))

		c.CloseIdleConnections()
	})

	It("should proxy HTTPS requests when cert is bad", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		// Cert has mismatched hostname.
		req, err := http.NewRequest(http.MethodGet, "https://books.toscrape.com", nil)
		Expect(err).To(BeNil())
		req.Header.Add("X-Cache-SSL", "INSECURE")

		resp, err := c.Do(req)
		Expect(err).To(BeNil())

		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

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

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		resp1, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		Expect(resp1.StatusCode).To(Equal(http.StatusOK))

		body1, err := ioutil.ReadAll(resp1.Body)
		Expect(err).To(BeNil())
		Expect(body1).To(ContainSubstring("Books"))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		resp2, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		Expect(resp2.StatusCode).To(Equal(http.StatusOK))

		body2, err := ioutil.ReadAll(resp2.Body)
		Expect(err).To(BeNil())
		// Expect(body2).To(ContainSubstring("Books"))
		Expect(body2).To(Equal(body1))

		ch2 := resp2.Header.Get("X-Cache")
		Expect(ch2).To(Equal("HIT"))

		c.CloseIdleConnections()
	})

	It("should proxy HTTPS requests (with bad cert) using the cache", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		// Cert has mismatched hostname.
		req1, err := http.NewRequest(http.MethodGet, "https://books.toscrape.com", nil)
		Expect(err).To(BeNil())
		req1.Header.Add("X-Cache-SSL", "INSECURE")

		resp1, err := c.Do(req1)
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		Expect(resp1.StatusCode).To(Equal(http.StatusOK))

		body1, err := ioutil.ReadAll(resp1.Body)
		Expect(err).To(BeNil())
		Expect(body1).To(ContainSubstring("Books"))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		// Cert has mismatched hostname.
		req2, err := http.NewRequest(http.MethodGet, "https://books.toscrape.com", nil)
		Expect(err).To(BeNil())
		req2.Header.Add("X-Cache-SSL", "INSECURE")

		resp2, err := c.Do(req2)
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		Expect(resp2.StatusCode).To(Equal(http.StatusOK))

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

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		resp1, err := c.Get("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		Expect(resp1.StatusCode).To(Equal(http.StatusOK))

		body1, err := ioutil.ReadAll(resp1.Body)
		Expect(err).To(BeNil())
		Expect(body1).To(ContainSubstring("Books"))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		resp2, err := c.Head("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		Expect(resp2.StatusCode).To(Equal(http.StatusOK))

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

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		resp1, err := c.Head("http://books.toscrape.com/")
		Expect(err).To(BeNil())

		defer resp1.Body.Close()

		Expect(resp1.StatusCode).To(Equal(http.StatusOK))

		ch1 := resp1.Header.Get("X-Cache")
		Expect(ch1).To(Equal("MISS"))

		length, err := strconv.Atoi(resp1.Header.Get("Content-Length"))
		Expect(err).To(BeNil())
		Expect(length).ToNot(BeZero())

		c.CloseIdleConnections()
	})

	It("should reject matching pages", func() {
		// fmt.Println(server.URL)

		// TODO This test should use a local test HTTP server?

		c, err := newProxyClient(proxyURL)
		Expect(err).To(BeNil())

		req, err := http.NewRequest(http.MethodGet, "http://books.toscrape.com", nil)
		Expect(err).To(BeNil())
		req.Header.Add("X-Cache-Reject", "p([a-z]+)ch")
		req.Header.Add("X-Cache-Reject", "\\<h3\\>.*\\</h3\\>")
		req.Header.Add("X-Cache-Reject", "Books")

		resp, err := c.Do(req)
		Expect(err).To(BeNil())

		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusPreconditionFailed))

		// ch := resp.Header.Get("X-Cache")
		// Expect(ch).To(Equal("MISS"))

		// Try again, expect the same result.
		resp2, err := c.Do(req)
		Expect(err).To(BeNil())

		defer resp2.Body.Close()

		Expect(resp2.StatusCode).To(Equal(http.StatusPreconditionFailed))

		// ch2 := resp2.Header.Get("X-Cache")
		// Expect(ch2).To(Equal("MISS"))

		c.CloseIdleConnections()
	})

	// XIt("should work with goproxy", func() {

	// 	cache = progszy.NewSqliteCache(testCachePath)
	// 	// server2 := httptest.NewServer(http.HandlerFunc(progszy.ProxyHandlerWith(cache)))
	// 	// fmt.Println("server started")

	// 	// ...
	// 	proxy := goproxy.NewProxyHttpServer()
	// 	proxy.Verbose = true
	// 	// log.Fatal(http.ListenAndServe(":8080", proxy))

	// 	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	// 	// TODO(js) See https://godoc.org/github.com/elazarl/goproxy - Should/can we simply use Do() with our existing handler?
	// 	proxy.OnRequest().DoFunc(
	// 		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	// 			log.Printf("OnRequest headers %v", r.Header)
	// 			log.Printf("OnRequest method %s", r.Method)

	// 			// resp, err := http.Get(r.URL.String())
	// 			resp, err := http.Get("http://books.toscrape.com")
	// 			if err != nil {
	// 				// TODO How do we handle errors in here? - By returning appropriate HTTP responses.
	// 				panic(err)
	// 			}
	// 			return nil, resp
	// 		})

	// 	// TODO(js) So we know our primary method's interface should just be: func(r *http.Request) *http.Response

	// 	// TODO Unbodge this.
	// 	server.Close()
	// 	server = httptest.NewServer(proxy)

	// 	c, err := newProxyClient2(server.URL)
	// 	Expect(err).To(BeNil())

	// 	// resp, err := c.Get("http://books.toscrape.com")
	// 	resp, err := c.Get("https://google.com")
	// 	Expect(err).To(BeNil())

	// 	defer resp.Body.Close()

	// 	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// 	body, err := ioutil.ReadAll(resp.Body)
	// 	Expect(err).To(BeNil())
	// 	Expect(body).To(ContainSubstring("Books"))

	// 	c.CloseIdleConnections()

	// 	// server2.Close()
	// 	// fmt.Println("server closed")
	// 	err = cache.CloseAll()
	// 	Expect(err).To(BeNil())

	// 	// // TODO(js) This is somewhat clunky.
	// 	// err = deleteSqliteDBs()
	// 	// Expect(err).To(BeNil())

	// })

})

var acceptAllCerts = &tls.Config{InsecureSkipVerify: true}

// var noProxyClient = &http.Client{Transport: &http.Transport{TLSClientConfig: acceptAllCerts}}

func newProxyClient(proxyURL string) (*http.Client, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: acceptAllCerts}
	client := &http.Client{Transport: tr}
	return client, nil
}

// func newProxyClient(proxyURL string) (*http.Client, error) {
// 	u, err := url.Parse(proxyURL)
// 	if err != nil {
// 		return nil, err
// 	}
// 	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
// 	return client, nil
// }

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
