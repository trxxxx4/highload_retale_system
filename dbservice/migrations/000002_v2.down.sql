ALTER TABLE users 
    ADD COLUMN password_hash TEXT,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
