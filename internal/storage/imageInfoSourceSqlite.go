package photofield

import (
	"log"
	. "photofield/internal"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type ImageInfoSourceSqlite struct {
	path    string
	pool    *sqlitex.Pool
	pending chan *ImageInfoSqlite
}

type ImageInfoSqlite struct {
	Path string
	ImageInfo
}

func NewImageInfoSourceSqlite() *ImageInfoSourceSqlite {

	var err error

	source := ImageInfoSourceSqlite{}
	source.path = "data/photofield.cache.db"
	source.ensureTable()
	source.pool, err = sqlitex.Open(source.path, 0, 10)
	if err != nil {
		panic(err)
	}

	source.pending = make(chan *ImageInfoSqlite, 100)
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

func (source *ImageInfoSourceSqlite) ensureTable() {
	conn := source.open()
	defer conn.Close()

	err := sqlitex.ExecScript(conn, `
		CREATE TABLE IF NOT EXISTS "infos" (
			"path" text,
			"width" integer,
			"height" integer,
			"datetime" datetime,
			"color" integer,
			PRIMARY KEY ("path")
		);
	`)
	if err != nil {
		panic(err)
	}

	// CREATE UNIQUE INDEX uix_infos_path ON infos(path);
}

func (source *ImageInfoSourceSqlite) writePendingInfosSqlite() {
	conn := source.open()
	defer conn.Close()

	stmt := conn.Prep(`
		INSERT INTO infos(path, width, height, datetime, color)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			datetime=excluded.datetime,
			color=excluded.color;`)
	defer stmt.Finalize()

	lastCommit := time.Now()
	inTransaction := false

	for imageInfo := range source.pending {
		if !inTransaction {
			err := sqlitex.Exec(conn, "BEGIN TRANSACTION;", nil)
			if err != nil {
				panic(err)
			}
			inTransaction = true
		}

		stmt.BindText(1, imageInfo.Path)
		stmt.BindInt64(2, (int64)(imageInfo.Width))
		stmt.BindInt64(3, (int64)(imageInfo.Height))
		stmt.BindText(4, imageInfo.DateTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
		stmt.BindInt64(5, (int64)(imageInfo.Color))
		_, err := stmt.Step()
		if err != nil {
			log.Printf("Unable to insert image info for %s: %s\n", imageInfo.Path, err.Error())
			continue
		}
		err = stmt.Reset()
		if err != nil {
			panic(err)
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

func (source *ImageInfoSourceSqlite) Get(path string) (*ImageInfo, error) {

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, datetime, color FROM infos
		WHERE path = ?;`)
	defer stmt.Finalize()

	stmt.BindText(1, path)

	exists, err := stmt.Step()
	if !exists {
		return nil, err
	}

	var imageInfo ImageInfo
	imageInfo.Width = stmt.ColumnInt(0)
	imageInfo.Height = stmt.ColumnInt(1)
	imageInfo.DateTime, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stmt.ColumnText(2))
	if err != nil {
		return &imageInfo, err
	}
	imageInfo.Color = (uint32)(stmt.ColumnInt64(3))

	if imageInfo.Width == 0 || imageInfo.Height == 0 {
		return nil, nil
	}
	if imageInfo.DateTime.IsZero() {
		return nil, nil
	}
	// if imageInfo.Color == 0 {
	// 	return nil, nil
	// }

	return &imageInfo, nil
}

func (source *ImageInfoSourceSqlite) Set(path string, info *ImageInfo) error {
	source.pending <- &ImageInfoSqlite{
		Path:      path,
		ImageInfo: *info,
	}
	return nil
}
