CREATE TABLE fingerprint (
    file_id INTEGER PRIMARY KEY REFERENCES infos(id) NOT NULL,
    hash_xxh64 INTEGER NOT NULL
);