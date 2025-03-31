-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- DB credentials table
CREATE TABLE db_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    db_host VARCHAR(255) NOT NULL,
    db_port INTEGER NOT NULL,
    db_name VARCHAR(255) NOT NULL,
    db_user VARCHAR(255) NOT NULL,
    db_password VARCHAR(255) NOT NULL,
    db_ssl_mode VARCHAR(50) NOT NULL DEFAULT 'disable',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexer types
CREATE TYPE indexer_type AS ENUM (
    'nft_bids',
    'nft_prices',
    'token_borrow',
    'token_prices'
);

-- Indexing status
CREATE TYPE indexer_status AS ENUM (
    'pending',
    'active',
    'paused',
    'failed',
    'completed'
);

-- Indexers table
CREATE TABLE indexers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    db_credential_id UUID NOT NULL REFERENCES db_credentials(id) ON DELETE CASCADE,
    indexer_type indexer_type NOT NULL,
    params JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_table VARCHAR(255) NOT NULL,
    webhook_id VARCHAR(255),
    status indexer_status NOT NULL DEFAULT 'pending',
    last_indexed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on user_id + indexer_type
CREATE INDEX idx_indexers_user_type ON indexers(user_id, indexer_type);

-- Create index on webhook_id
CREATE INDEX idx_indexers_webhook ON indexers(webhook_id);

-- Indexing logs table
CREATE TABLE indexing_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    indexer_id UUID NOT NULL REFERENCES indexers(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on indexer_id
CREATE INDEX idx_indexing_logs_indexer ON indexing_logs(indexer_id);