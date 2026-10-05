CREATE TABLE IF NOT EXISTS accounts(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    holder_id UUID NOT NULL REFERENCES users(id),
    removed BOOLEAN NOT NULL DEFAULT false
);
