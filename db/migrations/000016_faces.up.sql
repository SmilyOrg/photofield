CREATE TABLE face (
    id INTEGER PRIMARY KEY,
    file_id INTEGER NOT NULL REFERENCES infos(id),
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    w INTEGER NOT NULL,
    h INTEGER NOT NULL,
    confidence INTEGER NOT NULL,
    embedding BLOB NOT NULL
);

CREATE INDEX idx_face_file_id ON face(file_id);

ALTER TABLE infos ADD COLUMN face_count INTEGER DEFAULT NULL;
