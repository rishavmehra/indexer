-- Drop tables
DROP TABLE IF EXISTS indexing_logs;
DROP TABLE IF EXISTS indexers;
DROP TABLE IF EXISTS db_credentials;
DROP TABLE IF EXISTS users;

-- Drop types
DROP TYPE IF EXISTS indexer_status;
DROP TYPE IF EXISTS indexer_type;

-- Drop extension (optional, might be used by other applications)
-- DROP EXTENSION IF EXISTS "uuid-ossp";