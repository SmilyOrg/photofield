CREATE TABLE person (
    id INTEGER PRIMARY KEY,
    revision INTEGER NOT NULL DEFAULT 1,
    name TEXT UNIQUE,
    representative_face_id INTEGER,
    photo_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE face (
    id INTEGER PRIMARY KEY,
    file_id INTEGER NOT NULL REFERENCES infos(id),
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    w INTEGER NOT NULL,
    h INTEGER NOT NULL,
    confidence INTEGER NOT NULL,
    embedding BLOB NOT NULL,
    person_id INTEGER REFERENCES person(id)
);

CREATE INDEX idx_face_file_id ON face(file_id);
CREATE INDEX idx_face_person_id ON face(person_id);

-- Add foreign key constraint for representative_face_id after face table exists
-- SQLite doesn't support adding FK constraints after table creation, so it's defined but not enforced
