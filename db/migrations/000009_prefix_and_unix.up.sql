-- create prefix table
CREATE TABLE prefix (
  id integer primary key,
  str text unique
);

-- fill prefix table
INSERT INTO prefix(str)
SELECT
    rtrim(path, replace(replace(path, '/', ''), '\', '')) as str
FROM infos
WHERE true
GROUP BY str;

-- recreate table
ALTER TABLE infos RENAME TO old_infos;
CREATE TABLE infos (
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
	CONSTRAINT infos_pk PRIMARY KEY ("path_prefix_id", "filename")
);

-- reinsert with prefix columns
INSERT INTO infos
SELECT
    prefix.id as path_prefix_id,
    replace(path, rtrim(path, replace(replace(path, '/', ''), '\', '')), '') as filename,
    width,
    height,
    strftime("%s", created_at) as created_at_unix,
    (CASE WHEN substr(replace(created_at, rtrim(created_at, replace(created_at, " ", "")), ""), 1, 1) == "+" THEN 1 ELSE -1 END) * -- offset sign
    (
        CAST(substr(replace(created_at, rtrim(created_at, replace(created_at, " ", "")), ""), 2, 2) as INTEGER) * 60 + -- hour offset in minutes
        CAST(substr(replace(created_at, rtrim(created_at, replace(created_at, " ", "")), ""), 5, 2) as INTEGER) -- minute offset
    ) as created_at_tz_offset,
    color,
    orientation
FROM old_infos
JOIN prefix ON prefix.str == rtrim(path, replace(replace(path, '/', ''), '\', ''));

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

