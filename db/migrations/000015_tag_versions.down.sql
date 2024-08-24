PRAGMA foreign_keys=OFF;

-- Create a new table "new_tag" with the original format of table tag
CREATE TABLE new_tag (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE,
  updated_at_ms INTEGER
);

-- Transfer content from tag into new_tag
INSERT INTO new_tag SELECT * FROM tag;

-- Drop the new table tag
DROP TABLE tag;

-- Change the name of new_tag to tag
ALTER TABLE new_tag RENAME TO tag;

-- Enable foreign key constraints
PRAGMA foreign_key_check;

-- Reenable foreign key constraints
PRAGMA foreign_keys=ON;