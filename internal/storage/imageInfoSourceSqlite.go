package photofield

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	. "photofield/internal"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

type ImageInfoSourceSqlite struct {
	path    string
	pool    *sqlitex.Pool
	pending chan *ImageInfoWrite
}

type ImageInfoWriteType int32

const (
	AppendPath  ImageInfoWriteType = iota
	UpdateMeta  ImageInfoWriteType = iota
	UpdateColor ImageInfoWriteType = iota
)

type ImageInfoWrite struct {
	Path string
	Type ImageInfoWriteType
	ImageInfo
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
	if err != migrate.ErrNoChange {
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
		INSERT INTO infos(path, width, height, datetime)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			datetime=excluded.datetime;`)
	defer updateMeta.Finalize()

	updateColor := conn.Prep(`
		INSERT INTO infos(path, color)
		VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET
			color=excluded.color;`)
	defer updateColor.Finalize()

	insert := conn.Prep(`
		INSERT OR IGNORE INTO infos(path)
		VALUES (?)
	`)
	defer insert.Finalize()

	lastCommit := time.Now()
	inTransaction := false

	defer func() {
		err := sqlitex.Exec(conn, "COMMIT;", nil)
		if err != nil {
			panic(err)
		}
	}()

	for imageInfo := range source.pending {
		if !inTransaction {
			err := sqlitex.Exec(conn, "BEGIN TRANSACTION;", nil)
			if err != nil {
				panic(err)
			}
			inTransaction = true
		}

		switch imageInfo.Type {
		case AppendPath:
			insert.BindText(1, imageInfo.Path)
			_, err := insert.Step()
			if err != nil {
				log.Printf("Unable to insert path %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = insert.Reset()
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
			if err != nil {
				panic(err)
			}
			lastCommit = time.Now()
			inTransaction = false
		}
	}
}

func (source *ImageInfoSourceSqlite) Get(path string) (ImageInfo, bool) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, datetime, color FROM infos
		WHERE path = ?;`)
	defer stmt.Finalize()

	stmt.BindText(1, path)

	var imageInfo ImageInfo

	exists, _ := stmt.Step()
	if !exists {
		return imageInfo, false
	}

	imageInfo.Width = stmt.ColumnInt(0)
	imageInfo.Height = stmt.ColumnInt(1)
	imageInfo.DateTime, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(2))
	imageInfo.Color = (uint32)(stmt.ColumnInt64(3))
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

func (source *ImageInfoSourceSqlite) List(dir string, limit int) <-chan string {
	out := make(chan string)
	go func() {
		finished := Elapsed("listing sqlite")

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
			SELECT path
			FROM infos
			WHERE path LIKE ?
		`

		if limit > 0 {
			sql += `LIMIT ?`
		}

		sql += ";"

		stmt := conn.Prep(sql)
		defer stmt.Finalize()
		stmt.BindText(1, dir+"%")
		if limit > 0 {
			stmt.BindInt64(2, (int64)(limit))
		}

		for {
			if exists, err := stmt.Step(); err != nil {
				log.Printf("Error listing files: %s\n", err.Error())
			} else if !exists {
				break
			}
			out <- stmt.ColumnText(0)
		}

		finished()
		close(out)
	}()
	return out
}
