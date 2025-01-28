-- Create new file table structure
CREATE TABLE file (
  id INTEGER PRIMARY KEY,
  path_prefix_id INTEGER REFERENCES prefix(id),
  created_at_unix INTEGER,
  created_at_tz_offset INTEGER,
  width INTEGER,
  height INTEGER,
  orientation INTEGER,
  color INTEGER,
  latitude REAL,
  longitude REAL
);

-- Create identity table
CREATE TABLE identity (
  file_id INTEGER PRIMARY KEY REFERENCES file(id),
  filename TEXT NOT NULL,
  hash_xxh64 INTEGER,
  UNIQUE(hash_xxh64, filename)
);

-- Copy data
INSERT INTO file 
SELECT id, path_prefix_id, created_at_unix, created_at_tz_offset,
     width, height, orientation, color, latitude, longitude
FROM infos;

CREATE INDEX list_idx
ON file (
  path_prefix_id,
  created_at_unix ASC,
  created_at_tz_offset,
  width,
  height,
  orientation,
  color
);


INSERT INTO identity (file_id, filename, hash_xxh64)
SELECT id, filename, NULL FROM infos;

-- Update foreign keys
CREATE TABLE clip_emb_new (
  file_id INTEGER UNIQUE REFERENCES file(id),
  inv_norm INTEGER NOT NULL,
  embedding BLOB NOT NULL
);
INSERT INTO clip_emb_new SELECT * FROM clip_emb;
DROP TABLE clip_emb;
ALTER TABLE clip_emb_new RENAME TO clip_emb;

CREATE TABLE file_tag (
  tag_id INTEGER REFERENCES tag(id) NOT NULL,
  file_id INTEGER REFERENCES file(id) NOT NULL,
  len INTEGER NOT NULL,
  CONSTRAINT file_tag_pk UNIQUE (tag_id, file_id, len)
);
INSERT INTO file_tag SELECT * FROM infos_tag;

-- Clean up
DROP TABLE infos_tag;
DROP TABLE infos;
