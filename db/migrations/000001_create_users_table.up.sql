CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR NOT NULL UNIQUE,
    password VARCHAR NOT NULL,
    role INTEGER NOT NULL,
    removed BOOLEAN NOT NULL DEFAULT false
);
