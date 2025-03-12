CREATE TABLE thumb256_new (
    id INTEGER PRIMARY KEY,
    file_id INTEGER NOT NULL,
    created_at_unix INTEGER DEFAULT NULL,
    data BLOB DEFAULT NULL,
    timestamp_sec INTEGER NOT NULL DEFAULT 0,
    UNIQUE(file_id, timestamp_sec)
);

-- Copy data from the old table to the new table
INSERT INTO thumb256_new(file_id, created_at_unix, data)
SELECT id, created_at_unix, data FROM thumb256;

-- Drop the old table
DROP TABLE thumb256;

-- Rename the new table to the original name
ALTER TABLE thumb256_new RENAME TO thumb256;
