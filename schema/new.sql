CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email_address VARCHAR(255) NOT NULL,
    password VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE files (
    id SERIAL PRIMARY KEY,
    user_id SERIAL REFERENCES users(id),
    location VARCHAR(255) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
)