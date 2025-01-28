CREATE TABLE schema_migrations (version uint64,dirty bool);
CREATE TABLE IF NOT EXISTS "dirs" (
  "path" text,
  "indexed_at" datetime,
  PRIMARY KEY ("path")
);
CREATE TABLE prefix (
  id integer primary key,
  str text unique
);
CREATE TABLE infos (
	id INTEGER PRIMARY KEY,
	path_prefix_id INTEGER REFERENCES prefix(id),
	filename TEXT,
	width INTEGER,
	height INTEGER,
	created_at_unix INTEGER,
  	created_at_tz_offset INTEGER,
	color INTEGER,
	orientation INTEGER,
  	created_at TEXT GENERATED ALWAYS AS (
      datetime(
      created_at_unix + 
      created_at_tz_offset*60,
      'unixepoch'
    ) || ' ' ||
      -- timezone offset
    printf('%s%02d:%02d',
      (CASE WHEN created_at_tz_offset < 0 THEN '-' ELSE '+' END),
      abs(created_at_tz_offset)/60,
      abs(created_at_tz_offset) % 60
    )
  ) VIRTUAL, "latitude" REAL, "longitude" REAL,
	CONSTRAINT infos_pk UNIQUE ("path_prefix_id", "filename")
);
CREATE TABLE clip_emb (
	file_id INTEGER UNIQUE REFERENCES infos(id),
    inv_norm INTEGER NOT NULL,
    embedding BLOB NOT NULL
);
CREATE TABLE sqlite_stat1(tbl,idx,stat);
CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample);
CREATE TABLE infos_tag (
    tag_id INTEGER REFERENCES tag(id) NOT NULL,
    file_id INTEGER REFERENCES infos(id) NOT NULL,
    len INTEGER NOT NULL,
    CONSTRAINT infos_tag_pk UNIQUE (tag_id, file_id, len)
);
CREATE TABLE IF NOT EXISTS "tag" (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  updated_at_ms INTEGER,
  active BOOLEAN NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX version_unique ON schema_migrations (version);
CREATE INDEX list_idx
ON infos (
  path_prefix_id,
  created_at_unix ASC,
  created_at_tz_offset,
  width,
  height,
  orientation,
  color
);
