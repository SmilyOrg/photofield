CREATE INDEX list_paths_idx
ON infos (
  created_at COLLATE NOCASE,
  path COLLATE NOCASE,
  width,
  height,
  color
);

CREATE INDEX sorted_path_idx ON infos (path COLLATE NOCASE);
