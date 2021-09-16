package photofield

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	. "photofield/internal"
	"sync"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

type ImageInfoSourceSqlite struct {
	path             string
	pool             *sqlitex.Pool
	pending          chan *ImageInfoWrite
	transactionMutex sync.RWMutex
}

type ImageInfoWriteType int32

const (
	AppendPath  ImageInfoWriteType = iota
	UpdateMeta  ImageInfoWriteType = iota
	UpdateColor ImageInfoWriteType = iota
	Delete      ImageInfoWriteType = iota
)

type ImageInfoWrite struct {
	Path string
	Type ImageInfoWriteType
	ImageInfo
}

type ImageInfoExistence struct {
	SizeNull     bool
	DateTimeNull bool
	ColorNull    bool
}

type ImageInfoResult struct {
	ImageInfo
	ImageInfoExistence
}

type ImageInfoListResult struct {
	SourcedImageInfo
	ImageInfoExistence
}

func (info *ImageInfoExistence) NeedsMeta() bool {
	return info.SizeNull || info.DateTimeNull
}

func (info *ImageInfoExistence) NeedsColor() bool {
	return info.ColorNull
}

func NewImageInfoSourceSqlite(migrations embed.FS) *ImageInfoSourceSqlite {

	var err error

	source := ImageInfoSourceSqlite{}
	source.path = "data/photofield.cache.db"
	source.migrate(migrations)

	source.pool, err = sqlitex.Open(source.path, 0, 10)
	if err != nil {
		panic(err)
	}

	source.pending = make(chan *ImageInfoWrite, 100)
	go source.writePendingInfosSqlite()

	return &source
}

func (source *ImageInfoSourceSqlite) open() *sqlite.Conn {
	conn, err := sqlite.OpenConn(source.path, 0)
	if err != nil {
		panic(err)
	}
	return conn
}

func (source *ImageInfoSourceSqlite) migrate(migrations embed.FS) {

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

func (source *ImageInfoSourceSqlite) writePendingInfosSqlite() {
	conn := source.open()
	defer conn.Close()

	updateMeta := conn.Prep(`
		INSERT INTO infos(path, width, height, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
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
		VALUES (?)
	`)
	defer appendPath.Finalize()

	delete := conn.Prep(`
		DELETE FROM infos
		WHERE path = ?;`)
	defer delete.Finalize()

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
		case UpdateMeta:
			updateMeta.BindText(1, imageInfo.Path)
			updateMeta.BindInt64(2, (int64)(imageInfo.Width))
			updateMeta.BindInt64(3, (int64)(imageInfo.Height))
			updateMeta.BindText(4, imageInfo.DateTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
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

func (source *ImageInfoSourceSqlite) Get(path string) (ImageInfoResult, bool) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, created_at, color FROM infos
		WHERE path = ?;`)
	defer stmt.Finalize()

	stmt.BindText(1, path)

	var imageInfo ImageInfoResult

	exists, _ := stmt.Step()
	if !exists {
		return imageInfo, false
	}

	imageInfo.Width = stmt.ColumnInt(0)
	imageInfo.Height = stmt.ColumnInt(1)
	imageInfo.SizeNull = stmt.ColumnType(0) == sqlite.TypeNull || stmt.ColumnType(1) == sqlite.TypeNull

	imageInfo.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(2))
	imageInfo.DateTimeNull = stmt.ColumnType(2) == sqlite.TypeNull

	imageInfo.Color = (uint32)(stmt.ColumnInt64(3))
	imageInfo.ColorNull = stmt.ColumnType(3) == sqlite.TypeNull

	return imageInfo, true
}

func (source *ImageInfoSourceSqlite) Write(path string, info ImageInfo, writeType ImageInfoWriteType) error {
	source.pending <- &ImageInfoWrite{
		Path:      path,
		ImageInfo: info,
		Type:      writeType,
	}
	return nil
}

func (source *ImageInfoSourceSqlite) WaitForCommit() {
	source.transactionMutex.RLock()
	defer source.transactionMutex.RUnlock()
}

func (source *ImageInfoSourceSqlite) DeleteNonexistent(dir string, m map[string]struct{}) {
	source.WaitForCommit()
	for path := range source.ListPaths([]string{dir}, 0) {
		_, exists := m[path]
		if !exists {
			source.Write(path, ImageInfo{}, Delete)
		}
	}
	source.WaitForCommit()
}

func (source *ImageInfoSourceSqlite) List(dirs []string, options ListOptions) <-chan ImageInfoListResult {
	out := make(chan ImageInfoListResult, 10000)
	go func() {
		defer Elapsed("listing infos sqlite")()

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
			var info ImageInfoListResult
			info.Path = stmt.ColumnText(0)

			info.Width = stmt.ColumnInt(1)
			info.Height = stmt.ColumnInt(2)
			info.SizeNull = stmt.ColumnType(1) == sqlite.TypeNull || stmt.ColumnType(2) == sqlite.TypeNull

			info.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(3))
			info.DateTimeNull = stmt.ColumnType(3) == sqlite.TypeNull

			info.Color = (uint32)(stmt.ColumnInt64(4))
			info.ColorNull = stmt.ColumnType(4) == sqlite.TypeNull

			out <- info
		}

		close(out)
	}()
	return out
}

func (source *ImageInfoSourceSqlite) ListPaths(dirs []string, limit int) <-chan string {
	out := make(chan string, 10000)
	go func() {
		defer Elapsed("listing paths sqlite")()

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
