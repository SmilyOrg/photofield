CREATE TABLE clip_emb (
	file_id INTEGER UNIQUE REFERENCES infos(id),
    inv_norm INTEGER NOT NULL,
    embedding BLOB NOT NULL
);