ALTER TABLE infos RENAME TO old_infos;
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
      "unixepoch"
    ) || " " ||
      -- timezone offset
    printf("%s%02d:%02d",
      (CASE WHEN created_at_tz_offset < 0 THEN "-" ELSE "+" END),
      abs(created_at_tz_offset)/60,
      abs(created_at_tz_offset) % 60
    )
  ) VIRTUAL,
	CONSTRAINT infos_pk UNIQUE ("path_prefix_id", "filename")
);
INSERT INTO infos(
	id,
	path_prefix_id,
	filename,
	width,
	height,
	created_at_unix,
	created_at_tz_offset,
	color,
	orientation
)
	SELECT
		rowid,
		path_prefix_id,
		filename,
		width,
		height,
		created_at_unix,
		created_at_tz_offset,
		color,
		orientation
	FROM old_infos;
DROP TABLE old_infos;
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
