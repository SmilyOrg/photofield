ALTER TABLE infos DROP COLUMN orientation;

DROP INDEX list_paths_idx;

CREATE INDEX list_paths_idx
ON infos (
  created_at COLLATE NOCASE,
  path COLLATE NOCASE,
  width,
  height,
  color
);

