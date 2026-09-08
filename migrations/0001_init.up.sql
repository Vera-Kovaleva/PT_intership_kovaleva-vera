CREATE TABLE urls (
    short_code   VARCHAR(6)  NOT NULL,
    original_url TEXT        NOT NULL,
    url_hash     BYTEA       NOT NULL,

    CONSTRAINT urls_pkey         PRIMARY KEY (short_code),
    CONSTRAINT urls_url_hash_key UNIQUE      (url_hash)
);