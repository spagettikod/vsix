package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spagettikod/vsix/cli"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"golang.org/x/sync/errgroup"
)

var (
	ErrCacheNotFound = errors.New("object not found in cache")
)

const (
	CacheFilename           = "vsix.sqlite"
	sqlCreateExtensionTable = `
CREATE TABLE IF NOT EXISTS extension (
	id			INTEGER PRIMARY KEY AUTOINCREMENT,
	uid			TEXT NOT NULL UNIQUE,
	metadata	TEXT NOT NULL,
	updated_at	DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	created_at	DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
)`
	sqlCreateExtensionFTSTable = `
CREATE VIRTUAL TABLE IF NOT EXISTS extension_fts USING fts5 (
	id,
	query
)
`
	sqlCreateVersionTable = `
CREATE TABLE IF NOT EXISTS version (
	id			INTEGER PRIMARY KEY AUTOINCREMENT,
	uid			TEXT NOT NULL,
	version		TEXT NOT NULL
				GENERATED ALWAYS AS (
					json_extract(metadata, '$.version')
				),
	platform	TEXT NOT NULL
				GENERATED ALWAYS AS (
					COALESCE(json_extract(metadata, '$.targetPlatform'), 'universal')
				),
	tag			TEXT NOT NULL UNIQUE
				GENERATED ALWAYS AS (
					uid || '@' ||  version || ':' || platform
				),
	metadata	TEXT NOT NULL,
	updated_at	DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	created_at	DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	FOREIGN KEY (uid) REFERENCES extension(id)
)`

	sqlCreateQueryPreReleaseView = `
CREATE VIEW IF NOT EXISTS query_pre_release_vw AS
    SELECT
		ext.id AS extension_id,
		json_extract(ext.metadata, '$.extensionId') AS meta_extension_id,
        version.uid,
        version.tag,
        COALESCE(
            (
                SELECT json_extract(value, '$.value')
                FROM json_each(json(version.metadata), '$.properties')
                WHERE json_extract(value, '$.key') = 'Microsoft.VisualStudio.Code.PreRelease'
                LIMIT 1
            ),
            'false'
        ) AS pre_release,
		version.platform,
        json_extract(version.metadata, '$.lastUpdated') AS last_updated,
        (
            SELECT json_extract(value, '$.value')
            FROM json_each(json(ext.metadata), '$.statistics')
            WHERE json_extract(value, '$.statisticName') = 'install'
            LIMIT 1
        ) AS install,
        ROW_NUMBER() OVER (PARTITION BY version.uid ORDER BY datetime(json_extract(version.metadata, '$.lastUpdated')) DESC) AS version_rank
    FROM version AS version
    JOIN extension AS ext ON ext.uid = version.uid
`

	sqlCreateQueryView = `
CREATE VIEW IF NOT EXISTS query_vw AS
    SELECT
		ext.id AS extension_id,
		json_extract(ext.metadata, '$.extensionId') AS meta_extension_id, 
        version.uid,
        version.tag,
        COALESCE(
            (
                SELECT json_extract(value, '$.value')
                FROM json_each(json(version.metadata), '$.properties')
                WHERE json_extract(value, '$.key') = 'Microsoft.VisualStudio.Code.PreRelease'
                LIMIT 1
            ),
            'false'
        ) AS pre_release,
		version.platform,
        json_extract(version.metadata, '$.lastUpdated') AS last_updated,
        (
            SELECT json_extract(value, '$.value')
            FROM json_each(json(ext.metadata), '$.statistics')
            WHERE json_extract(value, '$.statisticName') = 'install'
            LIMIT 1
        ) AS install,
        ROW_NUMBER() OVER (PARTITION BY version.uid ORDER BY datetime(json_extract(version.metadata, '$.lastUpdated')) DESC) AS version_rank
    FROM version AS version
    JOIN extension AS ext ON ext.uid = version.uid
	WHERE pre_release = 'false'
`
)

type Cache struct {
	conn     *sql.DB
	Filename string
}

func OpenCache(filename string) (Cache, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf(("file:%s?_journal_mode=wal"), filename))
	if err != nil {
		return Cache{}, err
	}
	c := Cache{
		conn:     db,
		Filename: filename,
	}
	if err := c.create(); err != nil {
		return c, err
	}
	return c, nil
}

// create will create the database if it doesn't already exist.
func (c Cache) create() error {
	if _, err := c.conn.Exec(sqlCreateExtensionTable); err != nil {
		return err
	}
	if _, err := c.conn.Exec(sqlCreateExtensionFTSTable); err != nil {
		return err
	}
	if _, err := c.conn.Exec(sqlCreateVersionTable); err != nil {
		return err
	}
	if _, err := c.conn.Exec(sqlCreateQueryPreReleaseView); err != nil {
		return err
	}
	if _, err := c.conn.Exec(sqlCreateQueryView); err != nil {
		return err
	}
	return nil
}

// Reset removes all data and recreates the cache store.
func (c Cache) Reset() error {
	if _, err := c.conn.Exec("DROP TABLE IF EXISTS extension_fts"); err != nil {
		return err
	}
	if _, err := c.conn.Exec("DROP TABLE IF EXISTS extension"); err != nil {
		return err
	}
	if _, err := c.conn.Exec("DROP TABLE IF EXISTS version"); err != nil {
		return err
	}
	if _, err := c.conn.Exec("DROP VIEW IF EXISTS query_pre_release_vw"); err != nil {
		return err
	}
	if _, err := c.conn.Exec("DROP VIEW IF EXISTS query_vw"); err != nil {
		return err
	}
	return c.create()
}

// putExtension adds (or updates) the extension metadata into the cache. It
// also updates the full text search index accordingly.
func (c Cache) putExtension(uid vscode.UniqueID, metadata []byte) error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}

	metaUID := ""
	if err := tx.QueryRow(`SELECT json_extract(:metadata, '$.publisher.publisherName') || '.' ||
						json_extract(:metadata, '$.extensionName')`, sql.Named("metadata", metadata)).Scan(&metaUID); err != nil {
		return err
	}

	if metaUID != uid.String() {
		return fmt.Errorf("unique identifier does not the id fo the metadata, uid: %v metadata-uid: %v", uid.String(), metaUID)
	}

	var id int64
	err = tx.QueryRow(`INSERT INTO extension (uid, metadata)
					   VALUES (?, ?)
					   ON CONFLICT(uid) DO UPDATE
					   SET metadata = excluded.metadata,
						   updated_at = CURRENT_TIMESTAMP
					   RETURNING id`, uid.String(), metadata).Scan(&id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// --
	// Update the full text search index
	// --

	// use a select to extract the query string data for the full text search
	var q string
	if err = tx.QueryRow(`SELECT json_extract(:json, '$.extensionName') || ' ' ||
								 json_extract(:json, '$.displayName') || ' ' ||
								 json_extract(:json, '$.publisher.publisherName') || ' ' ||
								 json_extract(:json, '$.shortDescription')`, sql.Named("json", string(metadata))).Scan(&q); err != nil {
		tx.Rollback()
		return err
	}

	// remove the old search index
	if _, err := tx.Exec("DELETE FROM extension_fts WHERE id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	// insert the new full text search index value
	if _, err := tx.Exec("INSERT INTO extension_fts (id, query) VALUES (?, ?)", id, q); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (c Cache) putVersion(uid vscode.UniqueID, metadata []byte) error {
	_, err := c.conn.Exec(`INSERT INTO version (uid, metadata)
					   	   VALUES (?, ?)
					   	   ON CONFLICT(tag) DO UPDATE
					   	   SET metadata = excluded.metadata,
						   	   updated_at = CURRENT_TIMESTAMP`, uid.String(), metadata)
	return err
}

func (c Cache) ReindexP(bend Backend, p cli.Progresser) (int, int, error) {
	start := time.Now()
	slog.Debug("listing unique identifiers")

	go p.DoWork()
	// find all unique identifiers stored at the backend
	uids, err := bend.listUniqueID()
	p.StopWork()
	if err != nil {
		return 0, 0, fmt.Errorf("error listing unqiue identifiers from backend: %w", err)
	}
	slog.Debug("unique identifiers listed", "elapsedTime", time.Since(start).Round(time.Millisecond), "count", len(uids))
	p.Max(len(uids))

	// Create a buffered channel to limit concurrency to 5
	semaphore := make(chan struct{}, 20)

	// Use a mutex to safely update shared counters
	var mu sync.Mutex
	versionCount := 0

	// Use errgroup to manage parallel execution and error handling
	var g errgroup.Group

	p.Text("Indexing")
	for _, uid := range uids {
		// Capture loop variable to avoid closure issues
		currentUID := uid

		// Acquire semaphore slot
		semaphore <- struct{}{}

		g.Go(func() error {
			// Always release semaphore slot when done
			defer func() {
				p.Next()
				<-semaphore
			}()

			// Call IndexExtension
			count, err := c.IndexExtension(bend, currentUID)
			if err != nil {
				return err
			}
			// Safely update version count
			mu.Lock()
			versionCount += count
			mu.Unlock()

			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return 0, 0, err
	}

	return len(uids), versionCount, nil
}

func (c Cache) IndexExtension(bend Backend, uid vscode.UniqueID) (int, error) {
	b, err := bend.LoadExtensionMetadata(uid)
	if err != nil {
		return 0, fmt.Errorf("error loading metadata for %v: %w", uid.String(), err)
	}
	if err := c.putExtension(uid, b); err != nil {
		return 0, fmt.Errorf("error saving metadata to cache för %v: %w", uid.String(), err)
	}
	tags, err := bend.ListVersionTags(uid)
	if err != nil {
		return 0, fmt.Errorf("error listing version tags for %v: %w", uid.String(), err)
	}
	for _, tag := range tags {
		b, err := bend.LoadVersionMetadata(tag)
		if err != nil {
			return 0, fmt.Errorf("error loading version metadata for %v: %w", tag.String(), err)
		}
		if err := c.putVersion(uid, b); err != nil {
			return 0, fmt.Errorf("error saving version metadata to cache for %v: %w", tag.String(), err)
		}
	}
	return len(tags), nil
}

type OrderBy string

const (
	OrderByInstalls OrderBy = "install"
)

type Query struct {
	Text              string
	Platforms         []string
	IncludePreRelease bool
	Latest            bool
	SortOrder         OrderBy
}

func NewQuery() Query {
	return Query{
		Platforms:         []string{},
		IncludePreRelease: false,
		Latest:            true,
	}
}

func (q Query) quoteWrapPlatforms() []string {
	if q.Platforms == nil {
		return nil
	}
	wrapped := []string{}
	for _, p := range q.Platforms {
		wrapped = append(wrapped, fmt.Sprintf("'%s'", p))
	}
	return wrapped
}

func (q Query) andCondition(sql, c string) string {
	var prefix string
	if sql == "" {
		prefix = " WHERE "
	} else {
		prefix = " AND "
	}
	return fmt.Sprintf("%s %s %s ", sql, prefix, c)
}

func (q Query) ToSQL() string {
	sql := ""
	if q.Text != "" {
		sql = q.andCondition(sql, fmt.Sprintf("extension_id IN (SELECT id FROM extension_fts WHERE query MATCH '%s')", q.Text))
	}
	if len(q.Platforms) > 0 {
		sql = q.andCondition(sql, fmt.Sprintf("platform IN (%s)", strings.Join(q.quoteWrapPlatforms(), ", ")))
	}
	if q.Latest {
		sql = q.andCondition(sql, "version_rank = 1")
	}
	switch q.SortOrder {
	case OrderByInstalls:
		sql += "ORDER BY install DESC"
	default:
		sql += "ORDER BY uid, version_rank"
	}
	return sql
}

// QueryResult query result from a Query run against the cache.
type QueryResult struct {
	Tag         vscode.VersionTag
	LastUpdated time.Time
	Installs    int
}

func (c Cache) Query(q Query) ([]QueryResult, error) {
	sql := "SELECT tag, pre_release, last_updated, install FROM "
	if q.IncludePreRelease {
		sql += "query_pre_release_vw "
	} else {
		sql += "query_vw "
	}
	sql += q.ToSQL()
	slog.Debug("query generated the following sql statement", "sql", sql)
	rows, err := c.conn.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := []QueryResult{}
	for rows.Next() {
		r := QueryResult{}
		var strTag, strPreRelease, strDate string
		rows.Scan(&strTag, &strPreRelease, &strDate, &r.Installs)
		if r.Tag, err = vscode.ParseVersionTag(strTag); err != nil {
			return nil, err
		}
		if r.LastUpdated, err = time.Parse(time.RFC3339, strDate); err != nil {
			return nil, err
		}
		r.Tag.PreRelease = (strPreRelease == "true")
		res = append(res, r)
	}
	return res, nil
}

// List all extensions (and their versions) in the cache.
func (c Cache) List() ([]vscode.Extension, error) {
	rows, err := c.conn.Query("SELECT metadata FROM extension")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []string
	exts := []vscode.Extension{}
	for rows.Next() {
		var meta string
		if err := rows.Scan(&meta); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		metas = append(metas, meta)
	}

	// Create a buffered channel to limit concurrency to 5
	semaphore := make(chan struct{}, 20)
	// Use a mutex to safely update shared counters
	var mu sync.Mutex

	// Use errgroup to manage parallel execution and error handling
	var g errgroup.Group

	for _, meta := range metas {
		semaphore <- struct{}{}

		g.Go(func() error {
			defer func() {
				<-semaphore
			}()

			ext := vscode.Extension{}

			if err := json.Unmarshal([]byte(meta), &ext); err != nil {
				return fmt.Errorf("unmarshal failed: %w", err)
			}
			vers, err := c.listVersions(ext.UniqueID())
			if err != nil {
				return fmt.Errorf("error listing version for extension %s: %w", ext.UniqueID(), err)
			}
			ext.Versions = vers
			mu.Lock()
			exts = append(exts, ext)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return exts, nil
}

// listVersions lists all versions for the extension with the given identifier.
func (c Cache) listVersions(uid vscode.UniqueID) ([]vscode.Version, error) {
	rows, err := c.conn.Query("SELECT metadata FROM version WHERE uid = ?", uid.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meta string
	vers := []vscode.Version{}
	for rows.Next() {
		ver := vscode.Version{}
		if err := rows.Scan(&meta); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(meta), &ver); err != nil {
			return nil, err
		}
		vers = append(vers, ver)
	}
	return vers, nil
}

// ListVersionTags returns all version tags matching the given tag. For example
// the tag "redhat.java@1.0.0" will return all target platsform for that
// version. Another example is "redhat.java", it will return all versions and
// platforms for the extension.
func (c Cache) ListVersionTags(tag vscode.VersionTag) ([]vscode.VersionTag, error) {
	rows, err := c.conn.Query("SELECT tag FROM version WHERE tag LIKE ?", tag.String()+"%")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("ListVersionTag: tag not found", "tag", tag)
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	defer rows.Close()
	tags := []vscode.VersionTag{}
	for rows.Next() {
		stag := ""
		if err := rows.Scan(&stag); err != nil {
			return nil, err
		}
		tag, err := vscode.ParseVersionTag(stag)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// FindByVersionTag return the version matching the exact version tag.
func (c Cache) FindByVersionTag(tag vscode.VersionTag) (vscode.Version, error) {
	metadata := ""
	err := c.conn.QueryRow("SELECT metadata FROM version WHERE tag = ?", tag.String()).Scan(&metadata)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("FindByVersionTag: tag not found", "tag", tag)
			return vscode.Version{}, ErrCacheNotFound
		}
		return vscode.Version{}, err
	}
	v := vscode.Version{}
	return v, json.Unmarshal([]byte(metadata), &v)
}

func (c Cache) FindByUniqueID(uid vscode.UniqueID) (vscode.Extension, error) {
	metadata := ""
	err := c.conn.QueryRow("SELECT metadata FROM extension WHERE uid = ?", uid.String()).Scan(&metadata)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("FindByUniqueID: uid not found", "uid", uid.String())
			return vscode.Extension{}, ErrCacheNotFound
		}
		return vscode.Extension{}, err
	}
	ext := vscode.Extension{}
	if err := json.Unmarshal([]byte(metadata), &ext); err != nil {
		return vscode.Extension{}, err
	}

	// list a ll the versions for this extension
	vers, err := c.listVersions(uid)
	if err != nil {
		return vscode.Extension{}, err
	}
	ext.Versions = append(ext.Versions, vers...)
	return ext, nil
}

func (c Cache) ListPlatforms(uid vscode.UniqueID) ([]string, error) {
	rows, err := c.conn.Query("SELECT DISTINCT platform FROM query_pre_release_vw WHERE uid = ?", uid.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	platforms := []string{}

	for rows.Next() {
		p := ""
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		platforms = append(platforms, p)
	}

	return platforms, nil
}

func (c Cache) Exists(uid vscode.UniqueID, platform string) (bool, error) {
	v := ""
	if err := c.conn.QueryRow("SELECT DISTINCT id FROM version WHERE uid = ? AND platform = ?", uid.String(), platform).Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c Cache) Delete(tag vscode.VersionTag) error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}

	// make sure we remove the correct level of version, the incoming tag could
	// specify an exact platform, version or might be an entire extension
	if tag.HasTargetPlatform() || tag.HasVersion() {
		if tag.HasTargetPlatform() {
			_, err = tx.Exec("DELETE FROM version WHERE tag = ?", tag.String())
		} else {
			_, err = tx.Exec("DELETE FROM version WHERE version = ?", tag.Version)
		}
	} else {
		_, err = tx.Exec("DELETE FROM version WHERE uid = ?", tag.UniqueID.String())
	}
	if err != nil {
		return err
	}

	// if we've removed the last version, remove the extension
	uid := ""
	err = tx.QueryRow("SELECT uid FROM version WHERE uid = ?", tag.UniqueID.String()).Scan(&uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			tx.Exec("DELETE FROM extension WHERE uid = ?", tag.UniqueID.String())
		} else {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// Run will execute all aspects of a marketplace.Query against the database. This includes
// querying, sorting, limiting and paging.
func (c Cache) Run(q marketplace.Query) (vscode.Results, error) {
	res := vscode.NewResults()

	extensions := []vscode.Extension{}

	if !q.IsValid() {
		return res, marketplace.ErrInvalidQuery
	}

	if q.IsEmptyQuery() {
		// empty queries sorted by number of installs equates to a @popular query
		exts, err := c.List()
		if err != nil {
			return res, err
		}
		extensions = append(extensions, exts...)
	} else {
		uniqueIDs := q.CriteriaValues(marketplace.FilterTypeExtensionName)
		if len(uniqueIDs) > 0 {
			for _, uidstr := range uniqueIDs {
				uid, ok := vscode.Parse(uidstr)
				if !ok {
					return res, fmt.Errorf("invalid unique id in query %s", uidstr)
				}
				ext, err := c.FindByUniqueID(uid)
				if err != nil {
					return res, err
				}
				extensions = append(extensions, ext)
			}
		}

		searchValues := q.CriteriaValues(marketplace.FilterTypeSearchText)
		if len(searchValues) > 0 {
			exts, err := c.List()
			if err != nil {
				return res, err
			}
			for _, e := range exts {
				if e.MatchesQuery(searchValues...) {
					extensions = append(extensions, e)
				}
			}
		}

		searchValues = q.CriteriaValues(marketplace.FilterTypeExtensionID)
		if len(searchValues) > 0 {
			exts, err := c.List()
			if err != nil {
				return res, err
			}
			for _, e := range exts {
				if slices.Contains(searchValues, e.ID) {
					extensions = append(extensions, e)
				}
			}
		}
	}

	// set total count to all extensions found, before some might be removed if paginated
	res.SetTotalCount(len(extensions))

	// sort the result
	switch q.SortBy() {
	case marketplace.ByInstallCount:
		slices.SortFunc(extensions, vscode.SortFuncExtensionByInstallCount)
	case marketplace.ByName:
		slices.SortFunc(extensions, vscode.SortFuncExtensionByDisplayName)
	}

	// paginate
	begin, end := pageBoundaries(len(extensions), q.Filters[0].PageSize, q.Filters[0].PageNumber)

	// add sorted and paginated extensions to the result
	res.AddExtensions(extensions[begin:end])

	return res, nil
}

// pageBoundaries return the begin and end index for a given page size and page. Indices
// can be used when slicing arrays/slices.
func pageBoundaries(totalCount, pageSize, pageNumber int) (begin, end int) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	begin = ((pageNumber - 1) * pageSize)
	end = begin + pageSize
	if end > totalCount {
		end = totalCount
	}
	return
}

func (c Cache) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

type CacheStats struct {
	ExtensionCount int
	VersionCount   int
	Platforms      string
	LastUpdated    time.Time
}

func (c Cache) Stats() (CacheStats, error) {
	stats := CacheStats{}
	if err := c.conn.QueryRow("SELECT COUNT(id) FROM extension").Scan(&stats.ExtensionCount); err != nil {
		return stats, err
	}
	if err := c.conn.QueryRow("SELECT COUNT(id) FROM version").Scan(&stats.VersionCount); err != nil {
		return stats, err
	}
	if stats.VersionCount > 0 {
		if err := c.conn.QueryRow(`SELECT GROUP_CONCAT(platform, ', ') AS platforms FROM (SELECT DISTINCT platform FROM version ORDER BY platform) AS distinct_platforms`).Scan(&stats.Platforms); err != nil {
			return stats, err
		}
	}
	if stats.ExtensionCount > 0 {
		unixSec := int64(0)
		if err := c.conn.QueryRow(`SELECT strftime('%s', MAX(updated_at)) FROM (SELECT updated_at FROM extension UNION ALL SELECT updated_at FROM version) AS combined`).Scan(&unixSec); err != nil {
			return stats, err
		}
		stats.LastUpdated = time.Unix(unixSec, 0)
	}
	return stats, nil
}
