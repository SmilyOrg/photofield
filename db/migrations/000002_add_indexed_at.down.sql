ALTER TABLE infos RENAME COLUMN "created_at" TO "datetime";
ALTER TABLE infos DROP COLUMN indexed_at;
ALTER TABLE infos DROP COLUMN active;
