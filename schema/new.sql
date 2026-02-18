-- TABLE NAMES SHOULD BE PLURAL

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email_address VARCHAR(255) NOT NULL,
    password VARCHAR(128) NOT NULL,
    client_random_value VARCHAR(64) NOT NULL,
    wrapped_account_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT now(),
    allowed_storage BIGINT NOT NULL DEFAULT 1024000000
);
ALTER TABLE users OWNER TO gdrive;

CREATE TABLE files (
    id SERIAL PRIMARY KEY,
    user_id SERIAL REFERENCES users(id) ON DELETE CASCADE,
    location VARCHAR(255) NOT NULL,
    wrapped_file_key VARCHAR(128) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    file_type VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
ALTER TABLE files OWNER TO gdrive;

CREATE TABLE links (
    id SERIAL PRIMARY KEY,
    access_key VARCHAR(128) NOT NULL,
    access_count BIGINT NOT NULL,
    file_id SERIAL UNIQUE REFERENCES files(id) ON DELETE CASCADE,
    created_by SERIAL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
ALTER TABLE links OWNER TO gdrive;

CREATE TABLE password_reset_codes (
    id SERIAL PRIMARY KEY,
    user_id SERIAL REFERENCES users(id) ON DELETE CASCADE,
    reset_code VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now() 
);
ALTER TABLE password_reset_codes OWNER TO gdrive;