package sqlite

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"image/jpeg"
	"log"
	"net/http"
	"path/filepath"
	"photofield/internal/metrics"
	"photofield/io"
	"time"

	goio "io"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

var (
	ErrNotFound = fmt.Errorf("image not found")
)

type Source struct {
	path    string
	pool    *sqlitex.Pool
	pending chan Thumb
}

type Thumb struct {
	Id    uint32
	Bytes []byte
}

func (s *Source) Name() string {
	return "sqlite"
}

func (s *Source) DisplayName() string {
	return "Internal thumbnail"
}

func (s *Source) Ext() string {
	return ".jpg"
}

func (s *Source) GetDurationEstimate(size io.Size) time.Duration {
	return 879 * time.Microsecond // SSD
	// return 958 * time.Microsecond // HDD
}

func (s *Source) Rotate() bool {
	return false
}

func (s *Source) Size(size io.Size) io.Size {
	return io.Size{X: 256, Y: 256}.Fit(size, io.FitInside)
}

func New(path string, migrations embed.FS) *Source {

	var err error

	source := Source{
		path: path,
	}
	source.migrate(migrations)

	poolSize := 10
	source.pool, err = sqlitex.Open(source.path, 0, poolSize)
	if err != nil {
		panic(err)
	}
	conns := make([]*sqlite.Conn, poolSize)
	for i := 0; i < poolSize; i++ {
		conns[i] = source.pool.Get(context.Background())
		setPragma(conns[i], "synchronous", "NORMAL")
		assertPragma(conns[i], "synchronous", 1)
	}
	for i := 0; i < poolSize; i++ {
		source.pool.Put(conns[i])
	}

	source.pending = make(chan Thumb, 100)
	go source.writePending()

	return &source
}

func setPragma(conn *sqlite.Conn, name string, value interface{}) error {
	sql := fmt.Sprintf("PRAGMA %s = %v;", name, value)
	return sqlitex.ExecuteTransient(conn, sql, &sqlitex.ExecOptions{})
}

func assertPragma(conn *sqlite.Conn, name string, value interface{}) error {
	sql := fmt.Sprintf("PRAGMA %s;", name)
	return sqlitex.ExecuteTransient(conn, sql, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			expected := fmt.Sprintf("%v", value)
			actual := stmt.GetText(name)
			if expected != actual {
				return fmt.Errorf("unable to initialize %s to %v, got back %v", name, expected, actual)
			}
			return nil
		},
	})
}

func (s *Source) init() {
	flags := sqlite.OpenReadWrite |
		sqlite.OpenCreate |
		sqlite.OpenURI |
		sqlite.OpenNoMutex

	conn, err := sqlite.OpenConn(s.path, flags)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Optimized for 256px jpeg thumbnails
	pageSize := 16384
	err = setPragma(conn, "page_size", pageSize)
	if err != nil {
		panic(err)
	}

	// Vacuum to apply page size
	err = sqlitex.ExecuteTransient(conn, "VACUUM;", &sqlitex.ExecOptions{})
	if err != nil {
		panic(err)
	}

	err = assertPragma(conn, "page_size", pageSize)
	if err != nil {
		panic(err)
	}
}

func (s *Source) migrate(migrations embed.FS) {
	dbsource, err := httpfs.New(http.FS(migrations), "db/migrations-thumbs")
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("sqlite://%v", filepath.ToSlash(s.path))
	m, err := migrate.NewWithSourceInstance(
		"migrations-thumbs",
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

	if err == migrate.ErrNilVersion {
		s.init()
	}

	dirtystr := ""
	if dirty {
		dirtystr = " (dirty)"
	}
	log.Printf("thumbs database version %v%s, migrating if needed", version, dirtystr)

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

func (s *Source) Write(id uint32, bytes []byte) error {
	s.pending <- Thumb{
		Id:    id,
		Bytes: bytes,
	}
	return nil
}

func (s *Source) Delete(id uint32) error {
	s.pending <- Thumb{
		Id:    id,
		Bytes: nil,
	}
	return nil
}

func (s *Source) writePending() {
	c := s.pool.Get(context.Background())
	defer s.pool.Put(c)

	insert := c.Prep(`
		INSERT OR REPLACE INTO thumb256(id, created_at_unix, data)
		VALUES (?, ?, ?);`)
	defer insert.Reset()

	delete := c.Prep(`
		DELETE FROM thumb256 WHERE id = ?;`)
	defer delete.Reset()

	lastCommit := time.Now()
	lastOptimize := time.Time{}
	inTransaction := false

	for t := range s.pending {
		if !inTransaction {
			// s.transactionMutex.Lock()
			err := sqlitex.Execute(c, "BEGIN TRANSACTION;", nil)
			if err != nil {
				panic(err)
			}
			inTransaction = true
		}

		now := time.Now()

		if t.Bytes == nil {
			delete.BindInt64(1, int64(t.Id))
			_, err := delete.Step()
			if err != nil {
				log.Printf("Unable to delete image %d: %s\n", t.Id, err)
			}
			delete.Reset()
		} else {
			insert.BindInt64(1, int64(t.Id))
			insert.BindInt64(2, now.Unix())
			insert.BindBytes(3, t.Bytes)
			_, err := insert.Step()
			if err != nil {
				log.Printf("Unable to insert image %d: %s\n", t.Id, err)
			}
			insert.Reset()
		}

		sinceLastCommitSeconds := time.Since(lastCommit).Seconds()
		if inTransaction && (sinceLastCommitSeconds >= 10 || len(s.pending) == 0) {
			err := sqlitex.Execute(c, "COMMIT;", nil)
			lastCommit = time.Now()
			if err != nil {
				panic(err)
			}

			if time.Since(lastOptimize).Hours() >= 1 {
				lastOptimize = time.Now()
				log.Println("database optimizing")
				optimizeDone := metrics.Elapsed("database optimize")
				err = sqlitex.Execute(c, "PRAGMA optimize;", nil)
				if err != nil {
					panic(err)
				}
				optimizeDone()
			}

			// s.transactionMutex.Unlock()
			inTransaction = false
		}
	}
}

func (s *Source) Exists(ctx context.Context, id io.ImageId, path string) bool {
	exists := false
	s.Reader(ctx, id, path, func(r goio.ReadSeeker, err error) {
		if r != nil && err == nil {
			exists = true
		}
	})
	return exists
}

func (s *Source) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	c := s.pool.Get(ctx)
	defer s.pool.Put(c)

	stmt := c.Prep(`
		SELECT data
		FROM thumb256
		WHERE id == ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	exists, err := stmt.Step()
	if err != nil {
		return io.Result{Error: fmt.Errorf("unable to execute query: %w", err)}
	}
	if !exists {
		return io.Result{}
	}

	r := stmt.ColumnReader(0)
	return s.Decode(ctx, r)
}

func (s *Source) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	c := s.pool.Get(ctx)
	defer s.pool.Put(c)

	stmt := c.Prep(`
		SELECT data
		FROM thumb256
		WHERE id == ?;`)
	defer stmt.Reset()

	stmt.BindInt64(1, int64(id))

	exists, err := stmt.Step()
	if err != nil {
		fn(nil, fmt.Errorf("unable to execute query: %w", err))
		return
	}
	if !exists {
		fn(nil, ErrNotFound)
		return
	}

	r := stmt.ColumnReader(0)
	fn(r, nil)
}

func (s *Source) Decode(ctx context.Context, r goio.Reader) io.Result {
	img, err := jpeg.Decode(r)
	if err != nil {
		return io.Result{Error: fmt.Errorf("unable to decode image: %w", err)}
	}
	return io.Result{
		Image: img,
		Error: err,
	}
}

func (s *Source) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	var b bytes.Buffer
	return s.SetWithBuffer(ctx, id, path, &b, r)
}

func (s *Source) SetWithBuffer(ctx context.Context, id io.ImageId, path string, b *bytes.Buffer, r io.Result) bool {
	w := bufio.NewWriter(b)
	s.Encode(ctx, r, w)
	s.Write(uint32(id), b.Bytes())
	return true
}

func (s *Source) Encode(ctx context.Context, r io.Result, w goio.Writer) bool {
	if r.Image == nil || r.Error != nil {
		return false
	}
	bounds := r.Image.Bounds()
	if bounds.Dx() > 256 || bounds.Dy() > 256 {
		return false
	}

	jpeg.Encode(w, r.Image, &jpeg.Options{
		Quality: 70,
	})
	return true
}

// func (s *Source) migrate(migrations embed.FS) {
// 	dbsource, err := httpfs.New(http.FS(migrations), "db/migrations")
// 	if err != nil {
// 		panic(err)
// 	}
// 	url := fmt.Sprintf("sqlite://%v", filepath.ToSlash(s.path))
// 	m, err := migrate.NewWithSourceInstance(
// 		"migrations",
// 		dbsource,
// 		url,
// 	)
// 	if err != nil {
// 		panic(err)
// 	}

// 	version, dirty, err := m.Version()
// 	if err != nil && err != migrate.ErrNilVersion {
// 		panic(err)
// 	}

// 	dirtystr := ""
// 	if dirty {
// 		dirtystr = " (dirty)"
// 	}
// 	log.Printf("database version %v%s, migrating if needed", version, dirtystr)

// 	err = m.Up()
// 	if err != nil && err != migrate.ErrNoChange {
// 		panic(err)
// 	}

// 	serr, derr := m.Close()
// 	if serr != nil {
// 		panic(serr)
// 	}
// 	if derr != nil {
// 		panic(derr)
// 	}
// }

// pool, err := sqlitex.Open(path.Join(dir, "test/photofield.thumbs.db"), 0, 10)
// if err != nil {
// 	panic(err)
// }
// c := pool.Get(context.Background())
// defer pool.Put(c)

// stmt := c.Prep(`
// 	SELECT data
// 	FROM thumb256
// 	WHERE id == ?;`)
// defer stmt.Reset()

// maxid := int64(1000000)

// for i := 0; i < b.N; i++ {
// 	id := 1 + rand.Int63n(maxid)
// 	stmt.BindInt64(1, id)
// 	exists, err := stmt.Step()
// 	if err != nil {
// 		b.Error(err)
// 	}
// 	if !exists {
// 		b.Errorf("id not found: %d", id)
// 	}
// 	r := stmt.ColumnReader(1)
// 	io.ReadAll(r)
// 	stmt.Reset()
// }
