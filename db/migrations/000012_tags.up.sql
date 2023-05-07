CREATE TABLE tag (
    id INTEGER PRIMARY KEY,
    revision INTEGER NOT NULL,
    name TEXT UNIQUE
);

CREATE TABLE infos_tag (
    tag_id INTEGER REFERENCES tag(id) NOT NULL,
    file_id INTEGER REFERENCES infos(id) NOT NULL,
    len INTEGER NOT NULL,
    CONSTRAINT infos_tag_pk UNIQUE (tag_id, file_id, len)
);