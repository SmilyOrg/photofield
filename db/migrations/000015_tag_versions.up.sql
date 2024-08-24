PRAGMA foreign_keys=OFF;

-- Create a new table "new_tag" without the unique constraint
CREATE TABLE new_tag (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  updated_at_ms INTEGER,
  active BOOLEAN NOT NULL DEFAULT 1
);

-- Transfer content from tag into new_tag
INSERT INTO new_tag SELECT id, name, updated_at_ms, 1 FROM tag;

-- Drop the old table tag
DROP TABLE tag;

-- Change the name of new_tag to tag
ALTER TABLE new_tag RENAME TO tag;

PRAGMA foreign_key_check;

PRAGMA foreign_keys=ON;