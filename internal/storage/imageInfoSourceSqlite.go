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
	Path       string
	AppendOnly bool
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

	update := conn.Prep(`
		INSERT INTO infos(path, width, height, datetime, color)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			datetime=excluded.datetime,
			color=excluded.color;`)
	defer update.Finalize()

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

		if imageInfo.AppendOnly {
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
		} else {
			update.BindText(1, imageInfo.Path)
			update.BindInt64(2, (int64)(imageInfo.Width))
			update.BindInt64(3, (int64)(imageInfo.Height))
			update.BindText(4, imageInfo.DateTime.Format("2006-01-02 15:04:05.999999999 -0700 MST"))
			update.BindInt64(5, (int64)(imageInfo.Color))
			_, err := update.Step()
			if err != nil {
				log.Printf("Unable to insert image info for %s: %s\n", imageInfo.Path, err.Error())
				continue
			}
			err = update.Reset()
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

func (source *ImageInfoSourceSqlite) Set(path string, info ImageInfo) error {
	source.pending <- &ImageInfoSqlite{
		Path:      path,
		ImageInfo: info,
	}
	return nil
}

func (source *ImageInfoSourceSqlite) Add(path string) error {
	source.pending <- &ImageInfoSqlite{
		Path:       path,
		AppendOnly: true,
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
