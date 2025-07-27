package sqlite

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"photofield/internal/metrics"
	"photofield/io"
	"runtime/trace"
	"time"

	goio "io"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	"github.com/zelenko/go/54_rotate_image/rotate"
)

var (
	ErrNotFound = fmt.Errorf("image not found")
)

type Source struct {
	path      string
	pool      *sqlitex.Pool
	pending   chan Thumb
	closed    bool
	encodePng bool
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
	// return 879 * time.Microsecond // SSD
	return 958 * time.Microsecond // HDD
}

func (s *Source) Rotate() bool {
	return false
}

func (s *Source) Size(size io.Size) io.Size {
	return io.Size{X: 256, Y: 256}.Fit(size, io.FitInside)
}

//go:embed migrations/*.sql
var migrations embed.FS

func New(path string) *Source {

	var err error

	source := Source{
		path: path,
	}
	source.migrate()

	poolSize := 10
	source.pool, err = sqlitex.Open(source.path, 0, poolSize)
	if err != nil {
		panic(err)
	}
	conns := make([]*sqlite.Conn, poolSize)
	for i := 0; i < poolSize; i++ {
		conns[i], err = source.pool.Take(context.Background())
		if err != nil {
			panic(err)
		}
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

func (s *Source) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	close(s.pending)
	return s.pool.Close()
}

// Flush waits for all pending writes to complete
func (s *Source) Flush() {
	if s == nil || s.closed {
		return
	}
	// Send a marker to know when we've processed all pending items
	done := make(chan struct{})
	go func() {
		// Wait until the pending channel is empty
		for len(s.pending) > 0 {
			time.Sleep(time.Millisecond)
		}
		close(done)
	}()
	<-done
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

func (s *Source) migrate() {
	dbsource, err := httpfs.New(http.FS(migrations), "migrations")
	if err != nil {
		log.Printf("migrations not found, skipping: %v", err)
		return
	}
	url := fmt.Sprintf("sqlite://%v", filepath.ToSlash(s.path))
	m, err := migrate.NewWithSourceInstance(
		"migrations-thumbs",
		dbsource,
		url,
	)
	if err != nil {
		log.Fatalf("failed to create migrate instance for %s: %v", s.path, err)
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
	if version > 0 {
		log.Printf("thumbs database version %v%s, migrating if needed", version, dirtystr)
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
	c, err := s.pool.Take(context.Background())
	if err != nil {
		log.Printf("database write unable to get connection from pool: %s", err)
		return
	}
	defer s.pool.Put(c)

	if c == nil {
		log.Println("database write unable to get connection, stopping")
		return
	}

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
	defer trace.StartRegion(ctx, "sqlite.Get").End()
	c, err := s.pool.Take(ctx)
	if err != nil {
		return io.Result{Error: fmt.Errorf("unable to get connection from pool: %w", err)}
	}
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
	c, err := s.pool.Take(ctx)
	if err != nil {
		fn(nil, fmt.Errorf("unable to get connection from pool: %w", err))
		return
	}
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

func rotateImageIfNeeded(img image.Image, orientation io.Orientation) image.Image {
	switch orientation {
	case io.MirrorHorizontal:
		return rotate.FlipH(img)

	case io.Rotate180:
		return rotate.Rotate180(img)

	case io.MirrorVertical:
		return rotate.FlipV(img)

	case io.MirrorHorizontalRotate270:
		return rotate.Rotate90(rotate.FlipH(img))

	case io.Rotate90:
		return rotate.Rotate270(img)

	case io.MirrorHorizontalRotate90:
		return rotate.Rotate270(rotate.FlipH(img))

	case io.Rotate270:
		return rotate.Rotate90(img)

	}
	// Default case, return original image
	return img
}

func (s *Source) Encode(ctx context.Context, r io.Result, w goio.Writer) bool {
	if r.Image == nil || r.Error != nil {
		return false
	}

	img := rotateImageIfNeeded(r.Image, r.Orientation)
	bounds := img.Bounds()
	if bounds.Dx() > 256 || bounds.Dy() > 256 {
		return false
	}

	if s.encodePng {
		err := png.Encode(w, img)
		if err != nil {
			log.Printf("unable to encode image as PNG: %v", err)
			return false
		}
		return true
	}
	err := jpeg.Encode(w, img, &jpeg.Options{
		Quality: 70,
	})
	if err != nil {
		log.Printf("unable to encode image as JPEG: %v", err)
		return false
	}
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
