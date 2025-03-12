CREATE TABLE thumb256_new (
  id INTEGER PRIMARY KEY,
  created_at_unix INTEGER DEFAULT NULL,
  data BLOB DEFAULT NULL
);

-- Copy data from the current table to the new table
INSERT INTO thumb256_new(id, created_at_unix, data)
SELECT file_id, created_at_unix, data FROM thumb256 WHERE timestamp_sec = 0;

-- Drop the current table
DROP TABLE thumb256;

-- Rename the new table to the original name
ALTER TABLE thumb256_new RENAME TO thumb256;