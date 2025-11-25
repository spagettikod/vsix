package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	uid						TEXT NOT NULL UNIQUE,
	extension_id			TEXT NOT NULL
							GENERATED ALWAYS AS (
								json_extract(metadata, '$.extensionId')
							),
	display_name			TEXT NOT NULL
							GENERATED ALWAYS AS (
								json_extract(metadata, '$.displayName')
							),
	published_date			REAL NOT NULL
							GENERATED ALWAYS AS (
								datetime(json_extract(metadata, '$.publishedDate'), 'auto')
							),
	weighted_rating			REAL NOT NULL,
	install					INTEGER NOT NULL,
	metadata				TEXT NOT NULL,
	metadata_latest_version	TEXT NOT NULL,
	metadata_all_versions	TEXT NOT NULL,
	updated_at				TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	created_at				TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP)
) STRICT`
	sqlCreateExtensionFTSTable = `
CREATE VIRTUAL TABLE IF NOT EXISTS extension_fts USING fts5 (
	id,
	query
)`
	sqlCreateVersionTable = `
CREATE TABLE IF NOT EXISTS version (
	id				INTEGER PRIMARY KEY AUTOINCREMENT,
	extension_id	INTEGER NOT NULL,
	uid				TEXT NOT NULL,
	version			TEXT NOT NULL
					GENERATED ALWAYS AS (
						json_extract(metadata, '$.version')
					),
	platform		TEXT NOT NULL
					GENERATED ALWAYS AS (
						COALESCE(json_extract(metadata, '$.targetPlatform'), 'universal')
					),
	tag				TEXT NOT NULL UNIQUE
					GENERATED ALWAYS AS (
						uid || '@' ||  version || ':' || platform
					),
	last_updated 	REAL NOT NULL
				 	GENERATED ALWAYS AS (
				 		json_extract(metadata, '$.lastUpdated')
					),
	pre_release		TEXT NOT NULL,
	metadata		TEXT NOT NULL,
	updated_at		TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	created_at		TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	FOREIGN KEY (extension_id) REFERENCES extension(id)
) STRICT`
)

type Cache struct {
	conn     *sql.DB
	writeMux *sync.Mutex
	Filename string
}

func OpenCache(filename string) (Cache, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf(("file:%s?_journal_mode=wal"), filename))
	if err != nil {
		return Cache{}, err
	}
	c := Cache{
		conn:     db,
		writeMux: &sync.Mutex{},
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
	return c.create()
}

// putExtension adds (or updates) the extension metadata into the cache. It
// also updates the full text search index accordingly.
func (c Cache) putExtension(tx *sql.Tx, uid vscode.UniqueID, metadata, fullMetadata, latestMetadata []byte) (int, error) {
	metaUID := ""
	if err := tx.QueryRow(`SELECT LOWER(json_extract(:metadata, '$.publisher.publisherName')) || '.' ||
						LOWER(json_extract(:metadata, '$.extensionName'))`, sql.Named("metadata", metadata)).Scan(&metaUID); err != nil {
		return 0, err
	}

	if metaUID != uid.String() {
		return 0, fmt.Errorf("unique identifier does not match the id fo the metadata, uid: %v metadata-uid: %v", uid.String(), metaUID)
	}

	var weigthedRating float32
	if err := tx.QueryRow(`SELECT (
							SELECT 	json_extract(value, '$.value')
							FROM 	json_each(json(:json), '$.statistics')
							WHERE 	json_extract(value, '$.statisticName') = 'weightedRating'
							LIMIT 1
						   )`, sql.Named("json", string(metadata))).Scan(&weigthedRating); err != nil {
		return 0, err
	}
	var install float32
	if err := tx.QueryRow(`SELECT (
							SELECT 	json_extract(value, '$.value')
							FROM 	json_each(json(:json), '$.statistics')
							WHERE 	json_extract(value, '$.statisticName') = 'install'
							LIMIT 1
						   )`, sql.Named("json", string(metadata))).Scan(&install); err != nil {
		return 0, err
	}

	var id int64
	err := tx.QueryRow(`INSERT INTO extension (uid, metadata, weighted_rating, install, metadata_latest_version, metadata_all_versions)
					    VALUES (?, ?, ?, ?, ?, ?)
					    ON CONFLICT(uid) DO UPDATE
					    SET metadata = excluded.metadata,
						    updated_at = CURRENT_TIMESTAMP
					    RETURNING id`, uid.String(), string(metadata), weigthedRating, install, string(latestMetadata), string(fullMetadata)).Scan(&id)
	if err != nil {
		return 0, err
	}

	// --
	// Update the full text search index
	// --

	// use a select to extract the query string data for the full text search
	var q string
	if err = tx.QueryRow(`SELECT json_extract(:json, '$.extensionName') || ' ' ||
								 json_extract(:json, '$.displayName') || ' ' ||
								 json_extract(:json, '$.publisher.publisherName') || '.' || json_extract(:json, '$.extensionName') || ' ' ||
								 json_extract(:json, '$.publisher.publisherName') || ' ' ||
								 json_extract(:json, '$.shortDescription')`, sql.Named("json", string(metadata))).Scan(&q); err != nil {
		return 0, err
	}

	// remove the old search index
	if _, err := tx.Exec("DELETE FROM extension_fts WHERE id = ?", id); err != nil {
		return 0, err
	}

	// insert the new full text search index value
	if _, err := tx.Exec("INSERT INTO extension_fts (id, query) VALUES (?, ?)", id, q); err != nil {
		return 0, err
	}

	return int(id), nil
}

func (c Cache) putVersion(tx *sql.Tx, extensionId int, uid vscode.UniqueID, v vscode.Version) error {
	_, err := tx.Exec(`INSERT INTO version (extension_id, uid, metadata, pre_release)
					   VALUES (?, ?, ?, ?)
					   ON CONFLICT(tag) DO UPDATE
					   SET metadata = excluded.metadata,
					   	   pre_release = excluded.pre_release,
						   updated_at = CURRENT_TIMESTAMP`, extensionId, uid.String(), string(v.ToJSON()), v.IsPreRelease())
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
	extMeta, err := bend.LoadExtensionMetadata(uid)
	if err != nil {
		return 0, fmt.Errorf("error loading metadata for %v: %w", uid.String(), err)
	}
	ext := vscode.Extension{}
	if err := json.Unmarshal(extMeta, &ext); err != nil {
		return 0, fmt.Errorf("error unmarchaling extension metadata for %v: %w", uid.String(), err)
	}
	tags, err := bend.ListVersionTags(uid)
	if err != nil {
		return 0, fmt.Errorf("error listing version tags for %v: %w", uid.String(), err)
	}
	versions := vscode.Versions{}
	for _, tag := range tags {
		b, err := bend.LoadVersionMetadata(tag)
		if err != nil {
			return 0, fmt.Errorf("error loading version metadata for %v: %w", tag.String(), err)
		}
		v := vscode.Version{}
		if err := json.Unmarshal(b, &v); err != nil {
			return 0, fmt.Errorf("error unmarshaling version metadata for %v: %w", tag.String(), err)
		}
		versions = append(versions, v)
	}
	versions.Sort()
	// extension with all versions
	extFullMeta := ext
	extFullMeta.Versions = versions
	// extension with only the latest version
	extLatestMeta := ext
	extLatestMeta.Versions = versions.Latest(true)

	c.writeMux.Lock()
	defer c.writeMux.Unlock()
	tx, err := c.conn.Begin()
	if err != nil {
		return 0, err
	}
	id := 0
	if id, err = c.putExtension(tx, uid, extMeta, extFullMeta.ToJSON(), extLatestMeta.ToJSON()); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("error saving metadata to cache för %v: %w", uid.String(), err)
	}
	for _, v := range versions {
		if err := c.putVersion(tx, id, uid, v); err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("error saving version metadata to cache for %v: %w", uid.String(), err)
		}
	}
	return len(tags), tx.Commit()
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
		sql = q.andCondition(sql, fmt.Sprintf("e.id IN (SELECT id FROM extension_fts WHERE query MATCH '%s')", q.Text))
	}
	if len(q.Platforms) > 0 {
		sql = q.andCondition(sql, fmt.Sprintf("v.platform IN (%s)", strings.Join(q.quoteWrapPlatforms(), ", ")))
	}
	if !q.IncludePreRelease {
		sql = q.andCondition(sql, "v.pre_release = false ")
	}
	if q.Latest {
		sql += "GROUP BY v.uid "
	}
	switch q.SortOrder {
	case OrderByInstalls:
		sql += "ORDER BY e.install DESC"
	default:
		sql += "ORDER BY e.uid"
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
	var sql string
	if q.Latest {
		sql = "SELECT v.tag, v.pre_release, MAX(v.last_updated), e.install "
	} else {
		sql = "SELECT v.tag, v.pre_release, v.last_updated, e.install "
	}
	sql += "FROM extension AS e JOIN version AS v ON v.extension_id = e.id "
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
	err := c.conn.QueryRow("SELECT metadata_all_versions FROM extension WHERE uid = ?", uid.String()).Scan(&metadata)
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
	return ext, json.Unmarshal([]byte(metadata), &ext)
}

func (c Cache) ListPlatforms(uid vscode.UniqueID) ([]string, error) {
	rows, err := c.conn.Query("SELECT DISTINCT(platform) FROM version WHERE uid = ? ORDER BY platform", uid.String())
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

	var err error
	sqlStr := ""
	totalCount := 0
	args := []any{}
	limit := q.Filters[0].PageSize
	offset := (q.Filters[0].PageNumber - 1) * limit
	start := time.Now()
	if q.Flags.Is(marketplace.FlagIncludeLatestVersionOnly) {
		sqlStr = "SELECT e.metadata_latest_version "
	} else {
		sqlStr = "SELECT e.metadata_all_versions "
	}
	if q.IsEmptyQuery() {
		// empty queries sorted by number of installs equates to a @popular query
		sqlStr += "FROM extension AS e "
		if err := c.conn.QueryRow("SELECT COUNT(1) FROM extension").Scan(&totalCount); err != nil {
			return res, err
		}
	} else {
		extensionNames := q.CriteriaValues(marketplace.FilterTypeExtensionName)
		searchText := q.CriteriaValues(marketplace.FilterTypeSearchText)
		extensionIds := q.CriteriaValues(marketplace.FilterTypeExtensionID)
		if len(searchText) > 0 {
			sqlStr += `FROM extension_fts AS fts
					   JOIN extension AS e ON e.id = fts.id
					   WHERE fts.query MATCH ? `
			args = append(args, ftsArgs(searchText[0]))
			if err := c.conn.QueryRow(`SELECT COUNT(1)
    								   FROM extension_fts AS fts
									   JOIN extension AS e ON e.id = fts.id
									   WHERE fts.query MATCH ? `, ftsArgs(searchText[0])).Scan(&totalCount); err != nil {
				return res, err
			}
		} else if len(extensionIds) > 0 {
			for _, id := range extensionIds {
				args = append(args, id)
			}
			p := placeholders(len(extensionIds))
			sqlStr += `FROM extension AS e
					   WHERE e.extension_id IN `
			sqlStr = sqlStr + "(" + p + ") "
			if err := c.conn.QueryRow("SELECT COUNT(1) FROM extension WHERE extension_id IN ("+p+")", args...).Scan(&totalCount); err != nil {
				return res, err
			}
		} else if len(extensionNames) > 0 {
			for _, name := range extensionNames {
				args = append(args, name)
			}
			p := placeholders(len(extensionNames))
			sqlStr += `FROM extension AS e
					   WHERE e.uid IN `
			sqlStr = sqlStr + "(" + p + ") "
			if err := c.conn.QueryRow("SELECT COUNT(1) FROM extension WHERE uid IN ("+p+")", args...).Scan(&totalCount); err != nil {
				return res, err
			}
		} else {
			return res, fmt.Errorf("encountered an unsupported query type, query: %s", q.ToJSON())
		}
	}

	switch q.SortBy() {
	case marketplace.ByInstallCount:
		sqlStr += "ORDER BY e.install DESC "
	case marketplace.ByName:
		sqlStr += "ORDER BY e.display_name ASC "
	case marketplace.ByPublishedDate:
		sqlStr += "ORDER BY e.published_date DESC "
	case marketplace.ByRating:
		sqlStr += "ORDER BY e.weighted_rating DESC "
	case marketplace.ByNone:
	}
	sqlStr += "LIMIT ? OFFSET ?"

	args = append(args, limit, offset)

	slog.Debug("query to run", "sql", sqlStr, "args", args)
	rows, err := c.conn.Query(sqlStr, args...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		meta := ""
		if err := rows.Scan(&meta); err != nil {
			return res, err
		}
		ext := vscode.Extension{}
		if err := json.Unmarshal([]byte(meta), &ext); err != nil {
			return res, err
		}
		extensions = append(extensions, ext)
	}
	res.AddExtensions(extensions)
	// set total count to all extensions found, before some might be removed if paginated
	res.SetTotalCount(totalCount)
	slog.Debug("query done", "elapsedTime", time.Since(start).Truncate(time.Millisecond))
	return res, nil
}

func ftsArgs(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	in := strings.Split(s, " ")
	out := []string{}
	for _, spl := range in {
		spl = spl + "*"
		if strings.Contains(spl, "-") || strings.Contains(spl, ".") {
			spl = "\"" + spl + "\""
		}
		out = append(out, spl)
	}
	return strings.Join(out, " ")
}

func placeholders(count int) string {
	a := []string{}
	for i := 0; i < count; i++ {
		a = append(a, "?")
	}
	return strings.Join(a, ",")
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
