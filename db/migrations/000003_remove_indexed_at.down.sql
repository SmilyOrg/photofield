ALTER TABLE infos ADD COLUMN indexed_at DATETIME;
ALTER TABLE infos ADD COLUMN active BOOLEAN DEFAULT 1;