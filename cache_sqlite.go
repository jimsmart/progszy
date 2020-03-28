package progszy

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	// Import Sqlite db driver.
	_ "github.com/mattn/go-sqlite3"
)

// TODO(js) Cleanup...

// const defaultCachePath = "cache/"

// var cachePath string

// func setDefaultCachePath() error {
// 	path, err := os.Getwd()
// 	if err != nil {
// 		return err
// 	}
// 	cachePath = filepath.Join(path, defaultCachePath)
// 	return nil
// }

// func init() {
// 	err := setDefaultCachePath()
// 	if err != nil {
// 		panic(err)
// 	}
// }

const fileExt = ".sqlite"

type SqliteCache struct {
	path           string
	mu             sync.RWMutex
	dbByBaseDomain map[string]*sql.DB
}

// TODO(js) To prevent issues if/when rotating out an in-use db, perhaps we should have a RWMutex around each db?

// NewSqliteCache initialises and returns a new SqliteCache.
func NewSqliteCache(cachePath string) *SqliteCache {
	c := SqliteCache{
		path:           cachePath,
		dbByBaseDomain: make(map[string]*sql.DB),
	}
	return &c
}

// Get the cached response for the given URL.
// If the given URL does not exist in the cache,
// error ErrCacheMiss is returned.
func (c *SqliteCache) Get(uri string) (*CacheRecord, error) {

	// log.Println("Called Get")

	nurl, bd, err := cacheRecordKey(uri)
	if err != nil {
		return nil, err
	}

	db, err := c.getDB(bd)
	if err != nil {
		// log.Printf("cache.Get: getOrCreateDB error %s", err)
		return nil, err
	}
	if db == nil {
		// The db doesn't exist.
		// log.Println("cache.Get: getDB returned nil")
		return nil, ErrCacheMiss
	}

	r, err := fetchRecord(db, nurl)
	if err != nil {
		// log.Printf("cache.Get: fetchRecord error %s", err)
		return nil, err
	}
	if r == nil {
		// The record doesn't exist.
		// log.Println("cache.Get: record does not exist")
		return nil, ErrCacheMiss
	}

	return r, nil
}

func fetchRecord(db *sql.DB, nurl string) (*CacheRecord, error) {
	row := db.QueryRow(querySQL, nurl)
	r := CacheRecord{}
	err := row.Scan(&r.Key, &r.URL, &r.BaseDomain, &r.ContentLanguage, &r.ContentType, &r.ETag, &r.LastModified, &r.ZstdBody, &r.CompressedLength, &r.ContentLength, &r.ResponseTime, &r.MD5, &r.Created)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		// TODO(js) Improve error handling.
		return nil, err
	}
	return &r, nil
}

// Put adds the given URL/response pair to the cache.
func (c *SqliteCache) Put(cr *CacheRecord) error {

	// log.Println("Called Put")

	// r, err := newCacheRecord(uri, mime, etag, lastMod, b, responseTime)
	// if err != nil {
	// 	return err
	// }

	db, err := c.getOrCreateDB(cr.BaseDomain)
	if err != nil {
		return err
	}

	err = insertRecord(db, cr)
	// if err != nil {
	// 	log.Printf("insert error %v", err)
	// }
	return err
}

func insertRecord(db *sql.DB, r *CacheRecord) error {
	_, err := db.Exec(insertSQL, r.Key, r.URL, r.BaseDomain, r.ContentLanguage, r.ContentType, r.ETag, r.LastModified, r.ZstdBody, r.CompressedLength, r.ContentLength, r.ResponseTime, r.MD5, r.Created)
	return err
}

func (c *SqliteCache) CloseAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for bd, db := range c.dbByBaseDomain {
		err := db.Close()
		if err != nil {
			// TODO(js) Improve error handling.
			log.Printf("Error closing %s db: %v", bd, err)
		}
	}
	c.dbByBaseDomain = make(map[string]*sql.DB)
	return nil
}

// findDatabase()
//  - do we know its file location already?
//   - use domain-slug as map key, rlock the map first.
//   - yep, return resulting file location from the map.
//  - wlock the map, check again.
//   - yes, we have a file location - must've been another thread.
//   - return resulting file location from the map.
//  - keep the wlock
//  - find all files starting with domain-slug, ending with .sqlite
//   - if we found any:
//    - sort them, find the one with the newest timestamp in its name
//     - stick it into the wlocked map, and return it.
//   - otherwise, make a new filename from domain-slug and timestamp
//    - create an empty database with that filename.
//     - stick it into the wlocked map, and return it.

func (c *SqliteCache) getOrCreateDB(bd string) (*sql.DB, error) {
	// First we assume that a handle exists,
	// so we just try to get the existing handle.
	db, err := c.getDB(bd)
	if err != nil {
		return nil, err
	}
	if db != nil {
		return db, nil
	}
	// Not found, we must create a new db.
	return c.createDB(bd)
}

func (c *SqliteCache) getDB(bd string) (*sql.DB, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	db, ok := c.dbByBaseDomain[bd]
	if ok {
		return db, nil
	}

	// No database handle exists in the map.
	// Does a suitably named database already exist on the filesystem?

	db, err := c.findDB(bd)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (c *SqliteCache) createDB(bd string) (*sql.DB, error) {
	// Check with more expensive wlock.
	// (Assumes we've already checked with rlock alone.)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Do we have a handle already?
	db, ok := c.dbByBaseDomain[bd]
	if ok {
		// Yes, return it - must've come from another goroutine, inbetween the rlock and wlock.
		return db, nil
	}

	// No database handle exists in the map.
	// Does a suitably named database already exist on the filesystem?

	db, err := c.findDB(bd)
	if err != nil {
		return nil, err
	}
	if db != nil {
		return db, nil
	}

	// No, we must make a new db.
	filename := filepath.Join(c.path, bd+"-"+timestamp()+fileExt)
	db, err = createDB(filename)
	if err != nil {
		return nil, err
	}

	// Add the db handle to the map.
	c.dbByBaseDomain[bd] = db

	return db, nil
}

func (c *SqliteCache) findDB(bd string) (*sql.DB, error) {
	// (Assumes the mutex is already locked)

	filename, err := findSqliteFile(c.path, bd)
	if err != nil {
		return nil, err
	}

	if len(filename) == 0 {
		return nil, nil
	}

	// Yes, try using that.
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	// Add the db handle to the map.
	c.dbByBaseDomain[bd] = db
	return db, nil
}

func findSqliteFile(path, bd string) (string, error) {

	files, err := filterFiles(path, bd, fileExt)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", nil
	}

	// for _, file := range files {
	// 	fmt.Println(file)
	// }

	return files[len(files)-1], nil
}

func filterFiles(root, prefix, ext string) ([]string, error) {

	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}

	var files []string

	// We will filter files with the correct extension and name prefix.
	filterFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ext || !strings.HasPrefix(info.Name(), prefix) {
			return nil
		}

		files = append(files, path)
		return nil
	}

	err := filepath.Walk(root, filterFn)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	return files, nil
}

func timestamp() string {
	return time.Now().Format("2006-01-02-1504")
}

func createDB(filename string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	// Run db init DDL/scripts.
	for _, ddl := range createDDL {
		_, err = db.Exec(ddl)
		if err != nil {
			return nil, err
		}
	}

	// log.Printf("created db %s", filename)

	return db, nil
}

var createDDL = []string{`
	CREATE TABLE IF NOT EXISTS web_resource (
		normalised_url		TEXT NOT NULL,
		url					TEXT NOT NULL,
		base_domain			TEXT NOT NULL,
		content_language	TEXT NOT NULL,
		content_type		TEXT NOT NULL,
		etag				TEXT NOT NULL,
		last_modified		TEXT NOT NULL,
		content				BLOB,
		compressed_size		INTEGER NOT NULL,
		content_length		INTEGER NOT NULL,
		response_ms			REAL NOT NULL,
		md5					TEXT NOT NULL,
		created_at			DATETIME NOT NULL,
		PRIMARY KEY (normalised_url, content_language, content_type)
	)`, // TODO(js) Should etag and last_modified have be nullable?
	"CREATE INDEX IF NOT EXISTS idx_web_resource_url ON web_resource(url)",
	"CREATE INDEX IF NOT EXISTS idx_web_resource_created_at ON web_resource(created_at)",
}

const querySQL = "SELECT * FROM web_resource WHERE normalised_url = ?"

// TODO(js) Review/document this decision (replace vs ignore)
const insertSQL = "INSERT OR IGNORE INTO web_resource (normalised_url, url, base_domain, content_language, content_type, etag, last_modified, content, compressed_size, content_length, response_ms, md5, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)"

// const insertSQL = "INSERT INTO web_resource (normalised_url, url, base_domain, content_language, content_type, etag, last_modified, content, compressed_size, content_length, response_ms, md5, created_at) VALUES  (?,?,?,?,?,?,?,?,?,?,?,?,?)"
// const insertSQL = "INSERT OR REPLACE INTO web_resource (normalised_url, url, base_domain, content_language, content_type, etag, last_modified, content, compressed_size, content_length, response_ms, md5, created_at) VALUES  (?,?,?,?,?,?,?,?,?,?,?,?,?)"
