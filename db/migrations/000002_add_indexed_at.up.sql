ALTER TABLE infos RENAME COLUMN "datetime" TO "created_at"; 
ALTER TABLE infos ADD COLUMN indexed_at DATETIME;
ALTER TABLE infos ADD COLUMN active BOOLEAN DEFAULT 1;