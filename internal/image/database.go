package image

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"photofield/internal/clip"
	"photofield/internal/metrics"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
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
	UpdateAI    InfoWriteType = iota
	Delete      InfoWriteType = iota
	Index       InfoWriteType = iota
)

type InfoWrite struct {
	Path      string
	Id        ImageId
	Embedding clip.Embedding
	Type      InfoWriteType
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

type EmbeddingsResult struct {
	Id ImageId
	clip.Embedding
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

func (source *Database) writePendingInfosSqlite() {
	conn := source.open()
	defer conn.Close()

	upsertPrefix := conn.Prep(`
		INSERT OR IGNORE INTO prefix(str)
		VALUES (?);`)
	defer upsertPrefix.Finalize()

	updateMeta := conn.Prep(`
		INSERT INTO infos(path_prefix_id, filename, width, height, orientation, created_at_unix, created_at_tz_offset, latitude, longitude, location)
		SELECT
			id as path_prefix_id,
			? as filename,
			? as width,
			? as height,
			? orientation,
			? as created_at_unix,
			? as created_at_tz_offset,
			? as latitude,
			? as longitude,
			? as location
		FROM prefix
		WHERE str == ?
		ON CONFLICT(path_prefix_id, filename) DO UPDATE SET
			width=excluded.width,
			height=excluded.height,
			orientation=excluded.orientation,
			latitude=excluded.latitude,
			longitude=excluded.longitude,
			location=excluded.location,
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
		WHERE path_prefix_id == (
			SELECT id
			FROM prefix
			WHERE str == ?
		) AND filename == ?;`)
	defer delete.Finalize()

	upsertIndex := conn.Prep(`
		INSERT OR REPLACE INTO dirs(path, indexed_at)
		VALUES (?, ?);`)
	defer upsertIndex.Finalize()

	lastCommit := time.Now()
	lastOptimize := time.Time{}
	inTransaction := false

	defer func() {
		err := sqlitex.Execute(conn, "COMMIT;", nil)
		source.transactionMutex.Unlock()
		if err != nil {
			panic(err)
		}
	}()

	for imageInfo := range source.pending {
		if !inTransaction {
			source.transactionMutex.Lock()
			err := sqlitex.Execute(conn, "BEGIN TRANSACTION;", nil)
			if err != nil {
				panic(err)
			}
			inTransaction = true
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
		case UpdateMeta:
			dir, file := filepath.Split(imageInfo.Path)
			_, timezoneOffsetSeconds := imageInfo.DateTime.Zone()

			updateMeta.BindText(1, file)
			updateMeta.BindInt64(2, (int64)(imageInfo.Width))
			updateMeta.BindInt64(3, (int64)(imageInfo.Height))
			updateMeta.BindInt64(4, (int64)(imageInfo.Orientation))
			updateMeta.BindInt64(5, imageInfo.DateTime.Unix())
			updateMeta.BindInt64(6, int64(timezoneOffsetSeconds/60))
			updateMeta.BindFloat(7, imageInfo.Latitude)
			updateMeta.BindFloat(8, imageInfo.Longitude)
			updateMeta.BindText(9, imageInfo.Location)
			updateMeta.BindText(10, dir)

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
			dir, file := filepath.Split(imageInfo.Path)

			delete.BindText(1, dir)
			delete.BindText(2, file)

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

		}

		sinceLastCommitSeconds := time.Since(lastCommit).Seconds()
		if inTransaction && (sinceLastCommitSeconds >= 10 || len(source.pending) == 0) {
			err := sqlitex.Execute(conn, "COMMIT;", nil)
			lastCommit = time.Now()
			if err != nil {
				panic(err)
			}

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

	conn := source.pool.Get(nil)
	defer source.pool.Put(conn)

	stmt := conn.Prep(`
		SELECT width, height, orientation, color, created_at, location
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

	info.Location = stmt.ColumnText(5)

	return info, true
}

func (source *Database) GetBatch(ids []ImageId) <-chan InfoListResult {
	out := make(chan InfoListResult, 1000)
	go func() {

		conn := source.pool.Get(nil)
		defer source.pool.Put(conn)

		sql := `
		SELECT id, width, height, orientation, color, created_at_unix, created_at_tz_offset, location
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

			info.DateTime = time.Unix(unix, 0).In(time.FixedZone("tz_offset", timezoneOffset*60))
			info.DateTimeNull = stmt.ColumnType(5) == sqlite.TypeNull

			info.Location = stmt.ColumnText(7)

			out <- info
		}
		close(out)
	}()
	return out
}

func (source *Database) GetDir(dir string) (InfoResult, bool) {

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

func (source *Database) WriteAI(id ImageId, embedding clip.Embedding) error {
	source.pending <- &InfoWrite{
		Id:        id,
		Type:      UpdateAI,
		Embedding: embedding,
	}
	return nil
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

func (source *Database) DeleteNonexistent(dir string, paths map[string]struct{}) {
	for ip := range source.ListNonexistent(dir, paths) {
		source.Write(ip.Path, Info{}, Delete)
	}
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

		sql := `
			SELECT id, width, height, orientation, color, created_at_unix, created_at_tz_offset, location
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

		switch options.OrderBy {
		case None:
		case DateAsc:
			sql += `ORDER BY created_at_unix ASC `
		case DateDesc:
			sql += `ORDER BY created_at_unix DESC `
		default:
			panic("Unsupported listing order")
		}

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

			info.DateTime = time.Unix(unix, 0).In(time.FixedZone("tz_offset", timezoneOffset*60))
			info.DateTimeNull = stmt.ColumnType(5) == sqlite.TypeNull

			info.Location = stmt.ColumnText(7)

			out <- info
		}

		close(out)
	}()
	return out
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
			invnorm := uint16(clip.InvNormMean + stmt.ColumnInt64(1))

			size := stmt.ColumnLen(2)
			bytes := make([]byte, size)
			read := stmt.ColumnBytes(2, bytes)
			if read != size {
				log.Printf("Error reading embedding: buffer underrun, expected %d actual %d bytes\n", size, read)
				continue
			}

			out <- EmbeddingsResult{
				Id:        id,
				Embedding: clip.FromRaw(bytes, invnorm),
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
