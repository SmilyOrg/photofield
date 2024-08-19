package image

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"photofield/internal/clip"
	"photofield/internal/metrics"
	"photofield/search"
	"photofield/tag"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	"github.com/golang/geo/s2"
)

var dateFormat = "2006-01-02 15:04:05.999999 -07:00"

type ListOrder int32

const (
	None     ListOrder = iota
	DateAsc  ListOrder = iota
	DateDesc ListOrder = iota
)

type ListOptions struct {
	OrderBy ListOrder
	Limit   int
	Query   *search.Query
}

type DirsFunc func(dirs []string)
type stringSet map[string]struct{}

func (s *stringSet) Add(str string) {
	if _, ok := (*s)[str]; ok {
		return
	}
	(*s)[str] = struct{}{}
}

func (s *stringSet) Slice() []string {
	out := make([]string, 0, len(*s))
	for k := range *s {
		out = append(out, k)
	}
	return out
}

type Database struct {
	path             string
	pool             *sqlitex.Pool
	pending          chan *InfoWrite
	transactionMutex sync.RWMutex
	dirUpdateFuncs   []DirsFunc
}

type InfoWriteType int32

const (
	AppendPath    InfoWriteType = iota
	UpdateMeta    InfoWriteType = iota
	UpdateColor   InfoWriteType = iota
	UpdateAI      InfoWriteType = iota
	Delete        InfoWriteType = iota
	Index         InfoWriteType = iota
	AddTag        InfoWriteType = iota
	AddTagId      InfoWriteType = iota
	AddTagIds     InfoWriteType = iota
	RemoveTagIds  InfoWriteType = iota
	InvertTagIds  InfoWriteType = iota
	CompactTagIds InfoWriteType = iota
)

type InfoWrite struct {
	Path      string
	Id        int64
	Embedding clip.Embedding
	Type      InfoWriteType
	Ids       Ids
	Done      chan any
	Info
}

type InfoExistence struct {
	SizeNull        bool
	OrientationNull bool
	DateTimeNull    bool
	LatLngNull      bool
	ColorNull       bool
}

type InfoResult struct {
	Info
	InfoExistence
}

type InfoListResult struct {
	SourcedInfo
	InfoExistence
}

type EmbeddingsResult struct {
	Id ImageId
	clip.Embedding
}

type TagIdRange struct {
	Id tag.Id
	IdRange
}

type tagSet map[tag.Id]struct{}

func readEmbedding(stmt *sqlite.Stmt, invnormIndex int, embeddingIndex int) (clip.Embedding, error) {
	if stmt.ColumnType(invnormIndex) == sqlite.TypeNull || stmt.ColumnType(embeddingIndex) == sqlite.TypeNull {
		return clip.FromRaw(nil, 0), ErrNotFound
	}
	invnorm := uint16(clip.InvNormMean + stmt.ColumnInt64(invnormIndex))
	size := stmt.ColumnLen(embeddingIndex)
	bytes := make([]byte, size)
	read := stmt.ColumnBytes(embeddingIndex, bytes)
	if read != size {
		return clip.FromRaw(nil, 0), fmt.Errorf("unable to read embedding bytes, expected %d, got %d", size, read)
	}
	return clip.FromRaw(bytes, invnorm), nil
}

func (tags *tagSet) Add(id tag.Id) {
	(*tags)[id] = struct{}{}
}

func (tags *tagSet) Len() int {
	return len(*tags)
}

func (info *InfoExistence) NeedsMeta() bool {
	return info.SizeNull || info.OrientationNull || info.DateTimeNull
}

func (info *InfoExistence) NeedsColor() bool {
	return info.ColorNull
}

func NewDatabase(path string, migrations embed.FS) *Database {

	var err error

	source := Database{}
	source.path = path
	source.migrate(migrations)

	source.pool, err = sqlitex.Open(source.path, 0, 10)
	if err != nil {
		panic(err)
	}

	source.pending = make(chan *InfoWrite, 10000)
	go source.writePendingInfosSqlite()

	return &source
}

func (source *Database) Close() {
	if source == nil {
		return
	}
	source.dirUpdateFuncs = nil
	source.pool.Close()
	close(source.pending)
}

func (source *Database) open() *sqlite.Conn {
	conn, err := sqlite.OpenConn(source.path, 0)
	if err != nil {
		panic(err)
	}
	return conn
}

func (source *Database) vacuum() error {
	conn := source.open()
	defer conn.Close()

	log.Println("database vacuuming")
	defer metrics.Elapsed("database vacuum")()

	return sqlitex.Execute(conn, "VACUUM;", nil)
}

func (source *Database) migrate(migrations embed.FS) {
	dbsource, err := httpfs.New(http.FS(migrations), "db/migrations")
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("sqlite://%v", filepath.ToSlash(source.path))
	m, err := migrate.NewWithSourceInstance(
		"migrations",
		dbsource,
		url,
	)
	if err != nil {
		panic(err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		panic(err)
	}

	dirtystr := ""
	if dirty {
		dirtystr = " (dirty)"
	}
	log.Printf("cache database version %v%s, migrating if needed", version, dirtystr)

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		panic(err)
	}

	serr, derr := m.Close()
	if serr != nil {
		panic(serr)
	}
	if derr != nil {
		panic(derr)
	}
}

func (db *Database) HandleDirUpdates(fn DirsFunc) {
	db.dirUpdateFuncs = append(db.dirUpdateFuncs, fn)
}

func (source *Database) writePendingInfosSqlite() {
	conn := source.open()
	defer conn.Close()

	upsertPrefix := conn.Prep(`
		INSERT OR IGNORE INTO prefix(str)
		VALUES (?);`)
	defer upsertPrefix.Finalize()

	updateMeta := conn.Prep(`
		INSERT INTO infos(path_prefix_id, filename, width, height, orientation, created_at_unix, created_at_tz_offset, latitude, longitude)
		SELECT
			id as path_prefix_id,
			? as filename,
			? as width,
			? as height,
			? orientation,
			? as created_at_unix,
			? as created_at_tz_offset,
			? as latitude,
			? as longitude
		FROM prefix
		WHERE str == ?
		ON CONFLICT(path_prefix_id, filename) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			orientation=excluded.orientation,
			latitude=excluded.latitude,
			longitude=excluded.longitude,
			created_at_unix=excluded.created_at_unix,
			created_at_tz_offset=excluded.created_at_tz_offset;`)
	defer updateMeta.Finalize()

	updateColor := conn.Prep(`
		INSERT INTO infos(path_prefix_id, filename, color)
		SELECT
			id as path_prefix_id,
			? as filename,
			? as color
		FROM prefix
		WHERE str == ?
		ON CONFLICT(path_prefix_id, filename) DO UPDATE SET
			color=excluded.color;`)
	defer updateColor.Finalize()

	updateAI := conn.Prep(`
		INSERT OR REPLACE INTO clip_emb(file_id, inv_norm, embedding)
		VALUES (?, ?, ?);`)
	defer updateAI.Finalize()

	appendPath := conn.Prep(`
		INSERT OR IGNORE INTO infos(path_prefix_id, filename)
		SELECT
			id as path_prefix_id,
			? as filename
		FROM prefix
		WHERE str == ?`)
	defer appendPath.Finalize()

	delete := conn.Prep(`
		DELETE
		FROM infos
		WHERE id == ?;`)
	defer delete.Finalize()

	upsertIndex := conn.Prep(`
		INSERT OR REPLACE INTO dirs(path, indexed_at)
		VALUES (?, ?);`)
	defer upsertIndex.Finalize()

	upsertTag := conn.Prep(`
		INSERT OR IGNORE INTO tag(name, revision)
		VALUES (?, 1);`)
	defer upsertTag.Finalize()

	getTagId := conn.Prep(`	
		SELECT id
		FROM tag
		WHERE name = ?;`)
	defer getTagId.Finalize()

	deleteTagRange := conn.Prep(`
		DELETE FROM infos_tag
		WHERE tag_id == ? AND file_id == ? AND len == ?;`)
	defer deleteTagRange.Finalize()

	deleteTagRanges := conn.Prep(`
		DELETE FROM infos_tag
		WHERE tag_id == ?;`)
	defer deleteTagRanges.Finalize()

	insertTagRange := conn.Prep(`
		INSERT OR IGNORE INTO infos_tag (tag_id, file_id, len)
		VALUES (?, ?, ?);`)
	defer insertTagRange.Finalize()

	incrementTagRevision := conn.Prep(`
		UPDATE tag
		SET revision = revision + 1
		WHERE id == ?
		RETURNING revision;`)
	defer incrementTagRevision.Finalize()

	lastOptimize := time.Time{}
	inTransaction := false

	pendingCompactionTags := tagSet{}
	pendingUpdatedDirs := make(stringSet)

	defer func() {
		source.WaitForCommit()
	}()

	commitTicker := &time.Ticker{}
	commitInterval := 200 * time.Millisecond

	for {
		select {
		case <-commitTicker.C:
			if !inTransaction {
				commitTicker.Stop()
				continue
			}

			// Perform pending tag compaction
			if pendingCompactionTags.Len() > 0 {
				for id := range pendingCompactionTags {
					source.CompactTag(id)
				}
				pendingCompactionTags = tagSet{}
				continue
			}

			err := sqlitex.Execute(conn, "COMMIT;", nil)
			if err != nil {
				panic(err)
			}

			// Flush updated dirs
			dirs := pendingUpdatedDirs.Slice()
			for _, fn := range source.dirUpdateFuncs {
				fn(dirs)
			}
			clear(pendingUpdatedDirs)

			// Optimize if needed
			if time.Since(lastOptimize).Hours() >= 1 {
				lastOptimize = time.Now()
				log.Println("database optimizing")
				optimizeDone := metrics.Elapsed("database optimize")
				err = sqlitex.Execute(conn, "PRAGMA optimize;", nil)
				if err != nil {
					panic(err)
				}
				optimizeDone()
			}

			source.transactionMutex.Unlock()
			inTransaction = false

		case imageInfo, ok := <-source.pending:
			if !ok {
				log.Println("database closing")
				if inTransaction {
					err := sqlitex.Execute(conn, "COMMIT;", nil)
					if err != nil {
						panic(err)
					}
					source.transactionMutex.Unlock()
					inTransaction = false
				}
				return
			}

			if !inTransaction {
				source.transactionMutex.Lock()
				err := sqlitex.Execute(conn, "BEGIN TRANSACTION;", nil)
				if err != nil {
					panic(err)
				}
				inTransaction = true
				commitTicker = time.NewTicker(commitInterval)
			}

			switch imageInfo.Type {
			case AppendPath:
				dir, file := filepath.Split(imageInfo.Path)

				upsertPrefix.BindText(1, dir)
				_, err := upsertPrefix.Step()
				if err != nil {
					log.Printf("Unable to insert path prefix %s: %s\n", dir, err.Error())
					continue
				}
				err = upsertPrefix.Reset()
				if err != nil {
					panic(err)
				}

				appendPath.BindText(1, file)
				appendPath.BindText(2, dir)
				_, err = appendPath.Step()
				if err != nil {
					log.Printf("Unable to insert path filename %s: %s\n", file, err.Error())
					continue
				}
				err = appendPath.Reset()
				if err != nil {
					panic(err)
				}
				pendingUpdatedDirs.Add(dir)

			case UpdateMeta:
				dir, file := filepath.Split(imageInfo.Path)
				_, timezoneOffsetSeconds := imageInfo.DateTime.Zone()

				updateMeta.BindText(1, file)
				updateMeta.BindInt64(2, (int64)(imageInfo.Width))
				updateMeta.BindInt64(3, (int64)(imageInfo.Height))
				updateMeta.BindInt64(4, (int64)(imageInfo.Orientation))
				updateMeta.BindInt64(5, imageInfo.DateTime.Unix())
				updateMeta.BindInt64(6, int64(timezoneOffsetSeconds/60))
				if IsNaNLatLng(imageInfo.LatLng) {
					updateMeta.BindNull(7)
					updateMeta.BindNull(8)
				} else {
					updateMeta.BindFloat(7, imageInfo.LatLng.Lat.Degrees())
					updateMeta.BindFloat(8, imageInfo.LatLng.Lng.Degrees())
				}
				updateMeta.BindText(9, dir)

				_, err := updateMeta.Step()
				if err != nil {
					log.Printf("Unable to insert image info meta for %s: %s\n", imageInfo.Path, err.Error())
					continue
				}
				err = updateMeta.Reset()
				if err != nil {
					panic(err)
				}
				pendingUpdatedDirs.Add(dir)

			case UpdateColor:
				dir, file := filepath.Split(imageInfo.Path)

				updateColor.BindText(1, file)
				updateColor.BindInt64(2, (int64)(imageInfo.Color))
				updateColor.BindText(3, dir)

				_, err := updateColor.Step()
				if err != nil {
					log.Printf("Unable to insert image info meta for %s: %s\n", imageInfo.Path, err.Error())
					continue
				}
				err = updateColor.Reset()
				if err != nil {
					panic(err)
				}
				pendingUpdatedDirs.Add(dir)

			case UpdateAI:
				updateAI.BindInt64(1, int64(imageInfo.Id))
				updateAI.BindInt64(2, int64(imageInfo.Embedding.InvNormUint16())-clip.InvNormMean)
				updateAI.BindBytes(3, imageInfo.Embedding.Byte())

				_, err := updateAI.Step()
				if err != nil {
					log.Printf("Unable to insert image info ai for %d: %s\n", imageInfo.Id, err.Error())
					continue
				}
				err = updateAI.Reset()
				if err != nil {
					panic(err)
				}

			case Delete:
				id := ImageId(imageInfo.Id)

				// Delete image tags
				for r := range source.ListImageTagRanges(id) {
					ids := NewIds()
					ids.Add(r.IdRange)
					ids.SubtractInt(int(id))

					min := r.Low
					len := r.High - r.Low
					deleteTagRange.BindInt64(1, int64(r.Id))
					deleteTagRange.BindInt64(2, int64(min))
					deleteTagRange.BindInt64(3, int64(len))
					_, err := deleteTagRange.Step()
					if err != nil {
						log.Printf("Unable to delete tag range %d: %s\n", r.Id, err.Error())
						continue
					}
					err = deleteTagRange.Reset()
					if err != nil {
						panic(err)
					}

					for nr := range ids.RangeChan() {
						min := nr.Low
						len := nr.High - nr.Low
						insertTagRange.BindInt64(1, int64(r.Id))
						insertTagRange.BindInt64(2, int64(min))
						insertTagRange.BindInt64(3, int64(len))
						_, err := insertTagRange.Step()
						if err != nil {
							log.Printf("Unable to insert tag range %d: %s\n", r.Id, err.Error())
							continue
						}
						err = insertTagRange.Reset()
						if err != nil {
							panic(err)
						}
					}
				}

				// Delete image info
				delete.BindInt64(1, int64(id))
				_, err := delete.Step()
				if err != nil {
					log.Printf("Unable to delete path %s: %s\n", imageInfo.Path, err.Error())
					continue
				}
				err = delete.Reset()
				if err != nil {
					panic(err)
				}

			case Index:
				upsertIndex.BindText(1, imageInfo.Path)
				upsertIndex.BindText(2, imageInfo.DateTime.Format(dateFormat))
				_, err := upsertIndex.Step()
				if err != nil {
					log.Printf("Unable to set dir to indexed %s: %s\n", imageInfo.Path, err.Error())
					continue
				}
				err = upsertIndex.Reset()
				if err != nil {
					panic(err)
				}
				pendingUpdatedDirs.Add(imageInfo.Path)

			case AddTag:
				tagName := imageInfo.Path
				upsertTag.BindText(1, tagName)
				_, err := upsertTag.Step()
				if err != nil {
					log.Printf("Unable upsert tag %s: %s\n", tagName, err.Error())
					continue
				}
				err = upsertTag.Reset()
				if err != nil {
					panic(err)
				}
				close(imageInfo.Done)

			case AddTagId:
				tagName := imageInfo.Path
				upsertTag.BindText(1, tagName)
				_, err := upsertTag.Step()
				if err != nil {
					log.Printf("Unable upsert tag %s: %s\n", tagName, err.Error())
					continue
				}
				err = upsertTag.Reset()
				if err != nil {
					panic(err)
				}

				// Get tag id
				getTagId.BindText(1, tagName)
				ok, err := getTagId.Step()
				if err != nil {
					log.Printf("Unable to get tag id for %s: %s\n", tagName, err.Error())
					continue
				}
				if !ok {
					log.Printf("Unable to get tag id for %s, returned false\n", tagName)
					continue
				}
				tagId := tag.Id(getTagId.ColumnInt64(0))
				err = getTagId.Reset()
				if err != nil {
					panic(err)
				}

				// Add tag range
				min := imageInfo.Id
				len := 0
				insertTagRange.BindInt64(1, int64(tagId))
				insertTagRange.BindInt64(2, int64(min))
				insertTagRange.BindInt64(3, int64(len))
				_, err = insertTagRange.Step()
				if err != nil {
					log.Printf("Unable to insert tag %s: %s\n", tagName, err.Error())
					continue
				}
				err = insertTagRange.Reset()
				if err != nil {
					panic(err)
				}
				pendingCompactionTags.Add(tagId)

			case AddTagIds, RemoveTagIds, InvertTagIds, CompactTagIds:
				tagId := tag.Id(imageInfo.Id)

				ids := source.getTagImageIdsWithConn(conn, tagId)
				switch imageInfo.Type {
				case AddTagIds:
					ids.AddTree(imageInfo.Ids)
				case RemoveTagIds:
					ids.SubtractTree(imageInfo.Ids)
				case InvertTagIds:
					ids.InvertTree(imageInfo.Ids)
				case CompactTagIds:
					// Tags being compacted do not change
				default:
					panic("Unknown tag id diff type")
				}

				// Delete all tag ranges
				deleteTagRanges.BindInt64(1, int64(tagId))
				_, err := deleteTagRanges.Step()
				if err != nil {
					log.Printf("Unable to delete tag ranges %d: %s\n", tagId, err.Error())
					continue
				}
				err = deleteTagRanges.Reset()
				if err != nil {
					panic(err)
				}

				// Insert new tag ranges
				for r := range ids.RangeChan() {
					min := r.Low
					len := r.High - r.Low
					insertTagRange.BindInt64(1, int64(tagId))
					insertTagRange.BindInt64(2, int64(min))
					insertTagRange.BindInt64(3, int64(len))
					_, err := insertTagRange.Step()
					if err != nil {
						log.Printf("Unable to insert tag range %d: %s\n", tagId, err.Error())
						continue
					}
					err = insertTagRange.Reset()
					if err != nil {
						panic(err)
					}
				}

				// Increment tag revision
				incrementTagRevision.BindInt64(1, int64(tagId))
				ok, err := incrementTagRevision.Step()
				if err != nil {
					log.Printf("Unable to increment tag revision %d: %s\n", tagId, err.Error())
					continue
				}
				if !ok {
					panic("Unable to increment tag revision, returned false")
				}
				rev := incrementTagRevision.ColumnInt(0)
				err = incrementTagRevision.Reset()
				if err != nil {
					panic(err)
				}

				imageInfo.Done <- rev
				close(imageInfo.Done)
			}
		}

	}
}

func (source *Database) GetPathFromId(id ImageId) (string, bool) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT str || filename as path
		FROM infos
		JOIN prefix ON path_prefix_id == prefix.id
		WHERE infos.id == ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, (int64)(id))

	exists, _ := stmt.Step()
	if !exists {
		return "", false
	}

	return stmt.ColumnText(0), true
}

func (source *Database) Get(id ImageId) (InfoResult, bool) {

	conn := source.pool.Get(context.TODO())
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, orientation, color, created_at, latitude, longitude
		FROM infos
		WHERE id == ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, (int64)(id))

	var info InfoResult

	exists, _ := stmt.Step()
	if !exists {
		return info, false
	}

	info.Width = stmt.ColumnInt(0)
	info.Height = stmt.ColumnInt(1)
	info.SizeNull = stmt.ColumnType(0) == sqlite.TypeNull || stmt.ColumnType(1) == sqlite.TypeNull

	info.Orientation = Orientation(stmt.ColumnInt(2))
	info.OrientationNull = stmt.ColumnType(2) == sqlite.TypeNull

	info.Color = (uint32)(stmt.ColumnInt64(3))
	info.ColorNull = stmt.ColumnType(3) == sqlite.TypeNull

	info.DateTime, _ = time.Parse(dateFormat, stmt.ColumnText(4))
	info.DateTimeNull = stmt.ColumnType(4) == sqlite.TypeNull

	info.LatLngNull = stmt.ColumnType(5) == sqlite.TypeNull || stmt.ColumnType(6) == sqlite.TypeNull
	if info.LatLngNull {
		info.LatLng = NaNLatLng()
	} else {
		info.LatLng = s2.LatLngFromDegrees(stmt.ColumnFloat(5), stmt.ColumnFloat(6))
	}

	return info, true
}

func (source *Database) GetBatch(ids []ImageId) <-chan InfoListResult {
	out := make(chan InfoListResult, 1000)
	go func() {

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
		SELECT id, width, height, orientation, color, created_at_unix, created_at_tz_offset, latitude, longitude
		FROM infos
		WHERE id IN (`

		length := len(ids)
		if length > 1 {
			sql += strings.Repeat("?, ", length-1)
		}
		sql += `?);`

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		for i, id := range ids {
			stmt.BindInt64(1+i, (int64)(id))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}

			var info InfoListResult
			info.Id = (ImageId)(stmt.ColumnInt64(0))

			info.Width = stmt.ColumnInt(1)
			info.Height = stmt.ColumnInt(2)
			info.SizeNull = stmt.ColumnType(1) == sqlite.TypeNull || stmt.ColumnType(2) == sqlite.TypeNull

			info.Orientation = Orientation(stmt.ColumnInt(3))
			info.OrientationNull = stmt.ColumnType(3) == sqlite.TypeNull

			info.Color = (uint32)(stmt.ColumnInt64(4))
			info.ColorNull = stmt.ColumnType(4) == sqlite.TypeNull

			unix := stmt.ColumnInt64(5)
			timezoneOffset := stmt.ColumnInt(6)

			info.DateTime = time.Unix(unix, 0).In(time.FixedZone("", timezoneOffset*60))
			info.DateTimeNull = stmt.ColumnType(5) == sqlite.TypeNull

			info.LatLngNull = stmt.ColumnType(7) == sqlite.TypeNull || stmt.ColumnType(8) == sqlite.TypeNull
			if info.LatLngNull {
				info.LatLng = NaNLatLng()
			} else {
				info.LatLng = s2.LatLngFromDegrees(stmt.ColumnFloat(7), stmt.ColumnFloat(8))
			}

			out <- info
		}
		close(out)
	}()
	return out
}

func (source *Database) GetDir(dir string) (InfoResult, bool) {

	if source == nil {
		return InfoResult{}, false
	}

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT indexed_at FROM dirs
		WHERE path = ?;`)
	defer stmt.Reset()

	stmt.BindText(1, dir)

	var imageInfo InfoResult

	exists, _ := stmt.Step()
	if !exists {
		return imageInfo, false
	}

	imageInfo.DateTime, _ = time.Parse(dateFormat, stmt.ColumnText(0))
	imageInfo.DateTimeNull = stmt.ColumnType(0) == sqlite.TypeNull

	return imageInfo, true
}

func (source *Database) GetDirsCount(dirs []string) (int, bool) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	sql := `
	SELECT COUNT(id)
	FROM infos
	WHERE path_prefix_id IN (
		SELECT id
		FROM prefix
		WHERE
	`

	for i := range dirs {
		sql += `str LIKE ? `
		if i < len(dirs)-1 {
			sql += "OR "
		}
	}

	sql += `
		)
	`

	stmt := conn.Prep(sql)
	bindIndex := 1
	defer stmt.Reset()

	for _, dir := range dirs {
		stmt.BindText(bindIndex, dir+"%")
		bindIndex++
	}

	if exists, err := stmt.Step(); err != nil {
		log.Printf("error listing files: %s\n", err.Error())
	} else if !exists {
		return 0, false
	}

	return stmt.ColumnInt(0), true
}

func (source *Database) Write(path string, info Info, writeType InfoWriteType) error {
	source.pending <- &InfoWrite{
		Path: path,
		Info: info,
		Type: writeType,
	}
	return nil
}

func (source *Database) CompactTag(id tag.Id) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		d := make(chan any)
		source.pending <- &InfoWrite{
			Id:   int64(id),
			Type: CompactTagIds,
			Done: d,
		}
		<-d
		close(done)
	}()
	return done
}

func (source *Database) WriteTags(id ImageId, tags []tag.Tag) error {
	for _, t := range tags {
		source.pending <- &InfoWrite{
			Id:   int64(id),
			Type: AddTagId,
			Path: t.Name,
		}
	}
	return nil
}

func (source *Database) Delete(id ImageId) error {
	source.pending <- &InfoWrite{
		Id:   int64(id),
		Type: Delete,
	}
	return nil
}

func (source *Database) WriteAI(id ImageId, embedding clip.Embedding) error {
	source.pending <- &InfoWrite{
		Id:        int64(id),
		Type:      UpdateAI,
		Embedding: embedding,
	}
	return nil
}

func (source *Database) AddTag(name string) (<-chan struct{}, error) {
	d := make(chan any)
	done := make(chan struct{})
	source.pending <- &InfoWrite{
		Path: name,
		Type: AddTag,
		Done: d,
	}
	go func() {
		<-d
		source.WaitForCommit()
		close(done)
	}()
	return done, nil
}

func (source *Database) AddTagIds(id tag.Id, ids Ids) (int, error) {
	if ids.Len() == 0 {
		return source.GetTagRevision(id)
	}
	done := make(chan any)
	source.pending <- &InfoWrite{
		Id:   int64(id),
		Ids:  ids,
		Type: AddTagIds,
		Done: done,
	}
	rev := (<-done).(int)
	if rev == 0 {
		return source.GetTagRevision(id)
	} else {
		source.WaitForCommit()
		return rev, nil
	}
}

func (source *Database) RemoveTagIds(id tag.Id, ids Ids) (int, error) {
	if ids.Len() == 0 {
		return source.GetTagRevision(id)
	}
	done := make(chan any)
	source.pending <- &InfoWrite{
		Id:   int64(id),
		Ids:  ids,
		Type: RemoveTagIds,
		Done: done,
	}
	rev := (<-done).(int)
	if rev == 0 {
		return source.GetTagRevision(id)
	} else {
		source.WaitForCommit()
		return rev, nil
	}
}

func (source *Database) InvertTagIds(id tag.Id, ids Ids) (int, error) {
	if ids.Len() == 0 {
		return source.GetTagRevision(id)
	}
	done := make(chan any)
	source.pending <- &InfoWrite{
		Id:   int64(id),
		Ids:  ids,
		Type: InvertTagIds,
		Done: done,
	}
	rev := (<-done).(int)
	if rev == 0 {
		return source.GetTagRevision(id)
	} else {
		source.WaitForCommit()
		return rev, nil
	}
}

func (source *Database) GetTagImageIds(id tag.Id) Ids {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)
	return source.getTagImageIdsWithConn(conn, id)
}

func (source *Database) getTagImageIdsWithConn(conn *sqlite.Conn, id tag.Id) Ids {
	stmt := conn.Prep(`
	SELECT infos_tag.file_id, infos_tag.len
	FROM infos_tag
	JOIN tag ON infos_tag.tag_id = tag.id
	WHERE tag.id = ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	ids := NewIds()
	for {
		if exists, err := stmt.Step(); err != nil {
			log.Printf("Error listing files: %s\n", err.Error())
		} else if !exists {
			break
		}
		min := stmt.ColumnInt(0)
		len := stmt.ColumnInt(1)
		ids.Add(IdFromTo(min, min+len))
	}
	return ids
}

func (source *Database) ListTagRanges(id tag.Id) <-chan IdRange {
	out := make(chan IdRange, 100)
	go func() {
		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		stmt := conn.Prep(`
		SELECT infos_tag.file_id, infos_tag.len
		FROM infos_tag
		JOIN tag ON infos_tag.tag_id = tag.id
		WHERE tag.id = ?;`)
		defer stmt.Reset()

		stmt.BindInt64(1, int64(id))

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			min := stmt.ColumnInt(0)
			len := stmt.ColumnInt(1)
			out <- IdFromTo(min, min+len)
		}
		close(out)
	}()
	return out
}

func (source *Database) ListImageTagRanges(id ImageId) <-chan TagIdRange {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)
	return source.listImageTagRangesWithConn(conn, id)
}

func (source *Database) listImageTagRangesWithConn(conn *sqlite.Conn, id ImageId) <-chan TagIdRange {
	out := make(chan TagIdRange, 100)
	go func() {
		stmt := conn.Prep(`
		SELECT tag_id, file_id, len
		FROM infos_tag
		WHERE :file_id >= file_id AND :file_id <= file_id + len;`)
		defer stmt.Reset()

		stmt.BindInt64(1, int64(id))

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			tagId := tag.Id(stmt.ColumnInt(0))
			min := stmt.ColumnInt(1)
			len := stmt.ColumnInt(2)
			out <- TagIdRange{
				Id:      tagId,
				IdRange: IdFromTo(min, min+len),
			}
		}
		close(out)
	}()
	return out
}

func (source *Database) GetTagByName(name string) (tag.Tag, bool) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
	SELECT id, revision
	FROM tag
	WHERE name = ?;`)
	defer stmt.Reset()

	stmt.BindText(1, name)

	exists, _ := stmt.Step()
	if !exists {
		return tag.Tag{}, false
	}

	return tag.Tag{
		Id:       tag.Id(stmt.ColumnInt(0)),
		Name:     name,
		Revision: stmt.ColumnInt(1),
	}, true
}

func (source *Database) GetTagId(name string) (tag.Id, bool) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
	SELECT id
	FROM tag
	WHERE name = ?;`)
	defer stmt.Reset()

	stmt.BindText(1, name)

	exists, _ := stmt.Step()
	if !exists {
		return 0, false
	}

	return tag.Id(stmt.ColumnInt(0)), true
}

func (source *Database) GetTagName(id tag.Id) (string, bool) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
	SELECT name
	FROM tag
	WHERE id = ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	exists, _ := stmt.Step()
	if !exists {
		return "", false
	}

	return stmt.ColumnText(0), true
}

func (source *Database) GetTagRevision(id tag.Id) (int, error) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
	SELECT revision
	FROM tag
	WHERE id = ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	exists, _ := stmt.Step()
	if !exists {
		return 0, errors.New("tag not found")
	}

	return stmt.ColumnInt(0), nil
}

const defaultTagConditions string = `
	AND name NOT LIKE 'sys:%'
`

func (source *Database) ListImageTags(id ImageId) <-chan tag.Tag {
	out := make(chan tag.Tag, 100)
	go func() {
		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
		SELECT id, name, revision
		FROM infos_tag
		JOIN tag ON infos_tag.tag_id = tag.id
		WHERE :file_id >= file_id AND :file_id <= file_id + len
		`

		sql += defaultTagConditions

		sql += `
		ORDER BY length(name) ASC, name ASC;`

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		stmt.BindInt64(1, int64(id))

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing tags: %s\n", err.Error())
			} else if !exists {
				break
			}
			out <- tag.Tag{
				Id:       tag.Id(stmt.ColumnInt(0)),
				Name:     stmt.ColumnText(1),
				Revision: stmt.ColumnInt(2),
			}
		}
		close(out)
	}()
	return out
}

func (source *Database) ListTags(q string, limit int) <-chan tag.Tag {
	out := make(chan tag.Tag, 100)
	go func() {
		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
		SELECT id, name, revision
		FROM tag
		WHERE name LIKE ?
		`

		sql += defaultTagConditions

		sql += `
		ORDER BY name ASC
		LIMIT ?;`

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		stmt.BindText(1, "%"+q+"%")
		stmt.BindInt64(2, int64(limit))

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing tags: %s\n", err.Error())
			} else if !exists {
				break
			}
			out <- tag.Tag{
				Id:       tag.Id(stmt.ColumnInt(0)),
				Name:     stmt.ColumnText(1),
				Revision: stmt.ColumnInt(2),
			}
		}
		close(out)
	}()
	return out
}

func (source *Database) WaitForCommit() {
	source.transactionMutex.RLock()
	defer source.transactionMutex.RUnlock()
}

func (source *Database) ListNonexistent(dir string, paths map[string]struct{}) <-chan IdPath {
	source.WaitForCommit()
	out := make(chan IdPath, 1000)
	go func() {
		for ip := range source.ListIdPaths([]string{dir}, 0) {
			_, exists := paths[ip.Path]
			if !exists {
				out <- ip
			}
		}
		close(out)
	}()
	return out
}

func (source *Database) SetIndexed(dir string) {
	source.Write(dir, Info{
		DateTime: time.Now(),
	}, Index)
}

func (source *Database) List(dirs []string, options ListOptions) <-chan InfoListResult {
	out := make(chan InfoListResult, 1000)
	go func() {
		defer metrics.Elapsed("list infos sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := ""

		tags := options.Query.QualifierValues("tag")
		if len(tags) > 0 {
			sql += `
			WITH
			`
			for i := range tags {
				if i > 0 {
					sql += ","
				}
				sql += `
				tag` + strconv.Itoa(i) + ` AS (
					SELECT file_id, len
					FROM infos_tag
					WHERE tag_id IN (
						SELECT id
						FROM tag
						WHERE name = ?
					)
				)
				`
			}
		}

		sql += `
			SELECT infos.id, width, height, orientation, color, created_at_unix, created_at_tz_offset, latitude, longitude
			FROM infos
		`

		if len(tags) > 0 {
			for i := range tags {
				sql += fmt.Sprintf(`
					JOIN tag%[1]d ON id BETWEEN tag%[1]d.file_id AND tag%[1]d.file_id+tag%[1]d.len
				`, i)
			}
		}

		sql += `
			WHERE path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE `

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		switch options.OrderBy {
		case None:
		case DateAsc:
			sql += `
			ORDER BY created_at_unix ASC
			`
		case DateDesc:
			sql += `
			ORDER BY created_at_unix DESC
			`
		default:
			panic("Unsupported listing order")
		}

		if options.Limit > 0 {
			sql += `
				LIMIT ?
			`
		}

		sql += ";"

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		bindIndex := 1

		for _, tag := range tags {
			stmt.BindText(bindIndex, tag)
			bindIndex++
		}

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if options.Limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(options.Limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}

			var info InfoListResult
			info.Id = (ImageId)(stmt.ColumnInt64(0))

			info.Width = stmt.ColumnInt(1)
			info.Height = stmt.ColumnInt(2)
			info.SizeNull = stmt.ColumnType(1) == sqlite.TypeNull || stmt.ColumnType(2) == sqlite.TypeNull

			info.Orientation = Orientation(stmt.ColumnInt(3))
			info.OrientationNull = stmt.ColumnType(3) == sqlite.TypeNull

			info.Color = (uint32)(stmt.ColumnInt64(4))
			info.ColorNull = stmt.ColumnType(4) == sqlite.TypeNull

			unix := stmt.ColumnInt64(5)
			timezoneOffset := stmt.ColumnInt(6)

			info.DateTime = time.Unix(unix, 0).In(time.FixedZone("", timezoneOffset*60))
			info.DateTimeNull = stmt.ColumnType(5) == sqlite.TypeNull

			info.LatLngNull = stmt.ColumnType(7) == sqlite.TypeNull || stmt.ColumnType(8) == sqlite.TypeNull
			if info.LatLngNull {
				info.LatLng = NaNLatLng()
			} else {
				info.LatLng = s2.LatLngFromDegrees(stmt.ColumnFloat(7), stmt.ColumnFloat(8))
			}

			out <- info
		}

		close(out)
	}()
	return out
}

func (source *Database) ListWithEmbeddings(dirs []string, options ListOptions) <-chan InfoEmb {
	out := make(chan InfoEmb, 1000)
	go func() {
		defer metrics.Elapsed("list infos sqlite")()

		conn := source.pool.Get(context.TODO())
		defer source.pool.Put(conn)

		sql := ""

		sql += `
			SELECT infos.id, width, height, orientation, color, created_at_unix, created_at_tz_offset, latitude, longitude, inv_norm, embedding
			FROM infos
			LEFT JOIN clip_emb ON clip_emb.file_id = id
		`

		sql += `
			WHERE path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE `

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		switch options.OrderBy {
		case None:
		case DateAsc:
			sql += `
			ORDER BY created_at_unix ASC
			`
		case DateDesc:
			sql += `
			ORDER BY created_at_unix DESC
			`
		default:
			panic("Unsupported listing order")
		}

		if options.Limit > 0 {
			sql += `
				LIMIT ?
			`
		}

		sql += ";"

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		bindIndex := 1

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if options.Limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(options.Limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}

			var info InfoEmb
			info.Id = (ImageId)(stmt.ColumnInt64(0))

			info.Width = stmt.ColumnInt(1)
			info.Height = stmt.ColumnInt(2)
			info.Orientation = Orientation(stmt.ColumnInt(3))
			info.Color = (uint32)(stmt.ColumnInt64(4))

			unix := stmt.ColumnInt64(5)
			timezoneOffset := stmt.ColumnInt(6)

			info.DateTime = time.Unix(unix, 0).In(time.FixedZone("", timezoneOffset*60))

			if stmt.ColumnType(7) == sqlite.TypeNull || stmt.ColumnType(8) == sqlite.TypeNull {
				info.LatLng = NaNLatLng()
			} else {
				info.LatLng = s2.LatLngFromDegrees(stmt.ColumnFloat(7), stmt.ColumnFloat(8))
			}

			emb, err := readEmbedding(stmt, 9, 10)
			if err != nil {
				log.Printf("Error reading embedding for %d: %s\n", info.Id, err.Error())
			}
			info.Embedding = emb

			out <- info
		}

		close(out)
	}()
	return out
}

func (source *Database) GetImageEmbedding(id ImageId) (clip.Embedding, error) {
	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT inv_norm, embedding
		FROM clip_emb
		WHERE file_id = ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	if exists, err := stmt.Step(); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	invnorm := uint16(clip.InvNormMean + stmt.ColumnInt64(0))

	size := stmt.ColumnLen(1)
	bytes := make([]byte, size)
	read := stmt.ColumnBytes(1, bytes)
	if read != size {
		return nil, fmt.Errorf("error reading embedding: buffer underrun, expected %d actual %d bytes", size, read)
	}

	return clip.FromRaw(bytes, invnorm), nil
}

func (source *Database) ListEmbeddings(dirs []string, options ListOptions) <-chan EmbeddingsResult {
	out := make(chan EmbeddingsResult, 100)
	go func() {
		defer metrics.Elapsed("list embeddings sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT id, inv_norm, embedding
			FROM infos
			INNER JOIN clip_emb ON clip_emb.file_id = id
			WHERE path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE
		`

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		if options.Limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		defer stmt.Reset()

		bindIndex := 1
		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if options.Limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(options.Limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing embeddings: %s\n", err.Error())
			} else if !exists {
				break
			}

			id := (ImageId)(stmt.ColumnInt64(0))
			emb, err := readEmbedding(stmt, 1, 2)
			if err != nil {
				log.Printf("Error reading embedding: %s\n", err.Error())
				continue
			}

			out <- EmbeddingsResult{
				Id:        id,
				Embedding: emb,
			}
		}

		close(out)
	}()
	return out
}

func (source *Database) ListPaths(dirs []string, limit int) <-chan string {
	out := make(chan string, 10000)
	go func() {
		defer metrics.Elapsed("list paths sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT str || filename as path
			FROM infos
			JOIN prefix ON path_prefix_id == prefix.id
			WHERE path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE
		`

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		if limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Reset()

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			out <- stmt.ColumnText(0)
		}

		close(out)
	}()
	return out
}

func (source *Database) ListIdPaths(dirs []string, limit int) <-chan IdPath {
	out := make(chan IdPath, 10000)
	go func() {
		defer metrics.Elapsed("list id paths sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT infos.id, str || filename as path
			FROM infos
			JOIN prefix ON path_prefix_id == prefix.id
			WHERE path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE
		`

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		if limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Reset()

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			ip := IdPath{
				Id:   ImageId(stmt.ColumnInt64(0)),
				Path: stmt.ColumnText(1),
			}
			out <- ip
		}

		close(out)
	}()
	return out
}

func (source *Database) ListIds(dirs []string, limit int, missingEmbedding bool) <-chan ImageId {
	out := make(chan ImageId, 10000)
	go func() {
		defer metrics.Elapsed("list ids sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT id
			FROM infos
		`

		if missingEmbedding {
			sql += `LEFT JOIN clip_emb ON clip_emb.file_id = id`
		}

		sql += `
			WHERE
		`

		if missingEmbedding {
			sql += `file_id is NULL AND`
		}

		sql += `
			path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE
		`

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		if limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Reset()

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			out <- (ImageId)(stmt.ColumnInt64(0))
		}

		close(out)
	}()
	return out
}

func (source *Database) ListMissing(dirs []string, limit int, opts Missing) <-chan MissingInfo {
	out := make(chan MissingInfo, 1000)
	go func() {
		defer metrics.Elapsed("list missing sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT infos.id, str || filename as path`

		type condition struct {
			inputs []string
			output string
		}

		conds := make([]condition, 0)
		if opts.Metadata {
			conds = append(conds, condition{
				inputs: []string{
					"width",
					"height",
					"orientation",
					"created_at_unix",
				},
				output: "missing_metadata",
			})
		}
		if opts.Color {
			conds = append(conds, condition{
				inputs: []string{"color"},
				output: "missing_color",
			})
		}
		if opts.Embedding {
			conds = append(conds, condition{
				inputs: []string{"file_id"},
				output: "missing_embedding",
			})
		}

		for _, c := range conds {
			sql += `,
			`
			for i, input := range c.inputs {
				sql += fmt.Sprintf("%s IS NULL ", input)
				if i < len(c.inputs)-1 {
					sql += "OR "
				}
			}
			sql += fmt.Sprintf("AS %s", c.output)
		}

		sql += `
			FROM infos
			INNER JOIN prefix ON prefix.id = path_prefix_id
		`

		if opts.Embedding {
			sql += `
				LEFT JOIN clip_emb ON clip_emb.file_id = infos.id
			`
		}

		sql += `
			WHERE
			path_prefix_id IN (
				SELECT id
				FROM prefix
				WHERE
		`

		for i := range dirs {
			sql += `str LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		sql += `
			)
		`

		if len(conds) > 0 {
			sql += `
				AND (
			`

			for i, c := range conds {
				sql += fmt.Sprintf("%s ", c.output)
				if i < len(conds)-1 {
					sql += `OR 
					`
				}
			}
			// for i, c := range conds {
			// 	for j, input := range c.inputs {
			// 		sql += fmt.Sprintf("%s IS NULL ", input)
			// 		if j < len(c.inputs)-1 {
			// 			sql += "OR "
			// 		}
			// 	}
			// 	if i < len(conds)-1 {
			// 		sql += "OR "
			// 	}
			// 	sql += `
			// 	`
			// }
			sql += `
				)
			`
		}

		if limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Reset()

		for _, dir := range dirs {
			stmt.BindText(bindIndex, dir+"%")
			bindIndex++
		}

		if limit > 0 {
			stmt.BindInt64(bindIndex, (int64)(limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			r := MissingInfo{
				Id:   (ImageId)(stmt.ColumnInt64(0)),
				Path: stmt.ColumnText(1),
			}
			i := 2
			if opts.Color {
				r.Color = stmt.ColumnBool(i)
				i++
			}
			if opts.Embedding {
				r.Embedding = stmt.ColumnBool(i)
				i++
			}
			out <- r
		}

		close(out)
	}()
	return out
}
