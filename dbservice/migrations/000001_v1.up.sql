CREATE TABLE users(
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE products(
    id SERIAL PRIMARY KEY,
    sku TEXT NOT NULL UNIQUE, 
    name TEXT NOT NULL,
    price_cents INT NOT NULL CHECK (price_cents >= 0)
);
CREATE TABLE stock(
    product_id INT PRIMARY KEY REFERENCES products(id), 
    on_hand INT NOT NULL CHECK (on_hand >= 0), 
    reserved INT NOT NULL DEFAULT 0 CHECK (reserved >= 0), 
    CHECK (reserved <= on_hand)
);
CREATE TABLE reservations(
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id), 
    product_id INT NOT NULL REFERENCES products(id), 
    qty INT NOT NULL CHECK (qty > 0), 
    status TEXT NOT NULL CHECK (status IN ('active','paid','expired','cancelled')), 
    expires_at TIMESTAMPTZ NOT NULL, 
    idempotency_key TEXT NOT NULL, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(), 
    UNIQUE(user_id, idempotency_key)
);
CREATE INDEX ON reservations(status, expires_at);
CREATE TABLE payments(
    payment_id TEXT PRIMARY KEY,
    reservation_id UUID NOT NULL UNIQUE REFERENCES reservations(id), 
    amount_cents INT NOT NULL, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);