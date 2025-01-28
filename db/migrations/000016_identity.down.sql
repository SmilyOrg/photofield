-- Recreate infos and infos_tag tables
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
    printf('%s%02d:%02d',
      (CASE WHEN created_at_tz_offset < 0 THEN '-' ELSE '+' END),
      abs(created_at_tz_offset)/60,
      abs(created_at_tz_offset) % 60
    )
  ) VIRTUAL,
  latitude REAL,
  longitude REAL,
  CONSTRAINT infos_pk UNIQUE (path_prefix_id, filename)
);

CREATE TABLE infos_tag (
  tag_id INTEGER REFERENCES tag(id) NOT NULL,
  file_id INTEGER REFERENCES infos(id) NOT NULL,
  len INTEGER NOT NULL,
  CONSTRAINT infos_tag_pk UNIQUE (tag_id, file_id, len)
);

-- Copy data back to infos and infos_tag
INSERT INTO infos (id, path_prefix_id, created_at_unix, created_at_tz_offset, width, height, orientation, color, latitude, longitude, filename)
SELECT f.id, f.path_prefix_id, f.created_at_unix, f.created_at_tz_offset, f.width, f.height, f.orientation, f.color, f.latitude, f.longitude, i.filename
FROM file f
JOIN identity i ON f.id = i.file_id;

INSERT INTO infos_tag
SELECT * FROM file_tag;

-- Fix clip_emb references first
ALTER TABLE clip_emb RENAME TO clip_emb_temp;
CREATE TABLE clip_emb (
  file_id INTEGER UNIQUE REFERENCES infos(id),
  inv_norm INTEGER NOT NULL,
  embedding BLOB NOT NULL
);
INSERT INTO clip_emb SELECT * FROM clip_emb_temp;
DROP TABLE clip_emb_temp;

-- Drop new tables
DROP TABLE file_tag;
DROP TABLE identity;
DROP TABLE file;
