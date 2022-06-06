ALTER TABLE infos RENAME TO prefixed_infos;

CREATE TABLE "infos" (
	"path" text,
	"width" integer,
	"height" integer,
	"created_at" datetime,
	"color" integer, orientation INTEGER,
	PRIMARY KEY ("path")
);

CREATE INDEX list_paths_idx
ON infos (
  created_at COLLATE NOCASE,
  path COLLATE NOCASE,
  width,
  height,
  orientation,
  color
);

INSERT INTO infos
SELECT
	str || filename as path,
	width,
	height,
	created_at,
	color,
	orientation
FROM prefixed_infos
JOIN prefix ON prefix.id == prefixed_infos.path_prefix_id;

DROP TABLE prefix;
DROP TABLE prefixed_infos;
