package image

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"photofield/internal/metrics"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

type ListOrder int32

const (
	DateAsc  ListOrder = iota
	DateDesc ListOrder = iota
)

type ListOptions struct {
	OrderBy ListOrder
	Limit   int
}

type Database struct {
	path             string
	pool             *sqlitex.Pool
	pending          chan *InfoWrite
	transactionMutex sync.RWMutex
}

type InfoWriteType int32

const (
	AppendPath  InfoWriteType = iota
	UpdateMeta  InfoWriteType = iota
	UpdateColor InfoWriteType = iota
	Delete      InfoWriteType = iota
	Index       InfoWriteType = iota
)

type InfoWrite struct {
	Path string
	Type InfoWriteType
	Info
}

type InfoExistence struct {
	SizeNull        bool
	OrientationNull bool
	DateTimeNull    bool
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

func (info *InfoExistence) NeedsMeta() bool {
	return info.SizeNull || info.OrientationNull || info.DateTimeNull
}

func (info *InfoExistence) NeedsColor() bool {
	return info.ColorNull
}

func NewDatabase(migrations embed.FS) *Database {

	var err error

	source := Database{}
	source.path = "data/photofield.cache.db"
	source.migrate(migrations)

	source.pool, err = sqlitex.Open(source.path, 0, 10)
	if err != nil {
		panic(err)
	}

	source.pending = make(chan *InfoWrite, 100)
	go source.writePendingInfosSqlite()

	return &source
}

func (source *Database) open() *sqlite.Conn {
	conn, err := sqlite.OpenConn(source.path, 0)
	if err != nil {
		panic(err)
	}
	return conn
}

func (source *Database) migrate(migrations embed.FS) {

	dbsource, err := httpfs.New(http.FS(migrations), "db/migrations")
	if err != nil {
		panic(err)
	}
	m, err := migrate.NewWithSourceInstance(
		"migrations",
		dbsource,
		fmt.Sprintf("sqlite://%v", source.path),
	)
	if err != nil {
		panic(err)
	}
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

func (source *Database) writePendingInfosSqlite() {
	conn := source.open()
	defer conn.Close()

	updateMeta := conn.Prep(`
		INSERT INTO infos(path, width, height, orientation, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			orientation=excluded.orientation,
			created_at=excluded.created_at;`)
	defer updateMeta.Finalize()

	updateColor := conn.Prep(`
		INSERT INTO infos(path, color)
		VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET
			color=excluded.color;`)
	defer updateColor.Finalize()

	appendPath := conn.Prep(`
		INSERT OR IGNORE INTO infos(path)
		VALUES (?);`)
	defer appendPath.Finalize()

	delete := conn.Prep(`
		DELETE FROM infos
		WHERE path = ?;`)
	defer delete.Finalize()

	upsertIndex := conn.Prep(`
		INSERT OR REPLACE INTO dirs(path, indexed_at)
		VALUES (?, ?);`)
	defer upsertIndex.Finalize()

	lastCommit := time.Now()
	inTransaction := false

	defer func() {
		err := sqlitex.Exec(conn, "COMMIT;", nil)
		source.transactionMutex.Unlock()
		if err != nil {
			panic(err)
		}
	}()

	for imageInfo := range source.pending {
		if !inTransaction {
			source.transactionMutex.Lock()
			err := sqlitex.Exec(conn, "BEGIN TRANSACTION;", nil)
			if err != nil {
				panic(err)
			}
			inTransaction = true
		}

		switch imageInfo.Type {
		case AppendPath:
			appendPath.BindText(1, imageInfo.Path)
			_, err := appendPath.Step()
			if err != nil {
				log.Printf("Unable to insert path %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = appendPath.Reset()
			if err != nil {
				panic(err)
			}
		case UpdateMeta:
			updateMeta.BindText(1, imageInfo.Path)
			updateMeta.BindInt64(2, (int64)(imageInfo.Width))
			updateMeta.BindInt64(3, (int64)(imageInfo.Height))
			updateMeta.BindInt64(4, (int64)(imageInfo.Orientation))
			updateMeta.BindText(5, imageInfo.DateTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
			_, err := updateMeta.Step()
			if err != nil {
				log.Printf("Unable to insert image info meta for %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = updateMeta.Reset()
			if err != nil {
				panic(err)
			}
		case UpdateColor:
			updateColor.BindText(1, imageInfo.Path)
			updateColor.BindInt64(2, (int64)(imageInfo.Color))
			_, err := updateColor.Step()
			if err != nil {
				log.Printf("Unable to insert image info meta for %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = updateColor.Reset()
			if err != nil {
				panic(err)
			}

		case Delete:
			delete.BindText(1, imageInfo.Path)
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
			upsertIndex.BindText(2, imageInfo.DateTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
			_, err := upsertIndex.Step()
			if err != nil {
				log.Printf("Unable to set dir to indexed %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = upsertIndex.Reset()
			if err != nil {
				panic(err)
			}

		}

		sinceLastCommitMs := time.Since(lastCommit).Seconds()
		if inTransaction && (sinceLastCommitMs >= 10 || len(source.pending) == 0) {
			err := sqlitex.Exec(conn, "COMMIT;", nil)
			source.transactionMutex.Unlock()
			if err != nil {
				panic(err)
			}
			lastCommit = time.Now()
			inTransaction = false
		}
	}
}

func (source *Database) Get(path string) (InfoResult, bool) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, orientation, color, created_at FROM infos
		WHERE path = ?;`)
	defer stmt.Finalize()

	stmt.BindText(1, path)

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

	info.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(4))
	info.DateTimeNull = stmt.ColumnType(4) == sqlite.TypeNull

	return info, true
}

func (source *Database) GetDir(dir string) (InfoResult, bool) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT indexed_at FROM dirs
		WHERE path = ?;`)
	defer stmt.Finalize()

	stmt.BindText(1, dir)

	var imageInfo InfoResult

	exists, _ := stmt.Step()
	if !exists {
		return imageInfo, false
	}

	imageInfo.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(0))
	imageInfo.DateTimeNull = stmt.ColumnType(0) == sqlite.TypeNull

	return imageInfo, true
}

func (source *Database) Write(path string, info Info, writeType InfoWriteType) error {
	source.pending <- &InfoWrite{
		Path: path,
		Info: info,
		Type: writeType,
	}
	return nil
}

func (source *Database) WaitForCommit() {
	source.transactionMutex.RLock()
	defer source.transactionMutex.RUnlock()
}

func (source *Database) DeleteNonexistent(dir string, m map[string]struct{}) {
	source.WaitForCommit()
	for path := range source.ListPaths([]string{dir}, 0) {
		_, exists := m[path]
		if !exists {
			source.Write(path, Info{}, Delete)
		}
	}
}

func (source *Database) SetIndexed(dir string) {
	source.Write(dir, Info{
		DateTime: time.Now(),
	}, Index)
}

func (source *Database) List(dirs []string, options ListOptions) <-chan InfoListResult {
	out := make(chan InfoListResult, 10000)
	go func() {
		defer metrics.Elapsed("listing infos sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT path, width, height, orientation, color, created_at
			FROM infos
			WHERE 
		`

		for i := range dirs {
			sql += `path LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		switch options.OrderBy {
		case DateAsc:
			sql += `ORDER BY created_at ASC `
		case DateDesc:
			sql += `ORDER BY created_at DESC `
		default:
			panic("Unsupported listing order")
		}

		if options.Limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Finalize()

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
			info.Path = stmt.ColumnText(0)

			info.Width = stmt.ColumnInt(1)
			info.Height = stmt.ColumnInt(2)
			info.SizeNull = stmt.ColumnType(1) == sqlite.TypeNull || stmt.ColumnType(2) == sqlite.TypeNull

			info.Orientation = Orientation(stmt.ColumnInt(3))
			info.OrientationNull = stmt.ColumnType(3) == sqlite.TypeNull

			info.Color = (uint32)(stmt.ColumnInt64(4))
			info.ColorNull = stmt.ColumnType(4) == sqlite.TypeNull

			info.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(5))
			info.DateTimeNull = stmt.ColumnType(5) == sqlite.TypeNull

			out <- info
		}

		close(out)
	}()
	return out
}

func (source *Database) ListPaths(dirs []string, limit int) <-chan string {
	out := make(chan string, 10000)
	go func() {
		defer metrics.Elapsed("listing paths sqlite")()

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT path, width, height, created_at, color
			FROM infos
			WHERE 
		`

		for i := range dirs {
			sql += `path LIKE ? `
			if i < len(dirs)-1 {
				sql += "OR "
			}
		}

		if limit > 0 {
			sql += `LIMIT ? `
		}

		sql += ";"

		stmt := conn.Prep(sql)
		bindIndex := 1
		defer stmt.Finalize()

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
