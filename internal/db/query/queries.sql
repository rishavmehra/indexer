-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateDBCredential :one
INSERT INTO db_credentials (
    user_id,
    db_host,
    db_port,
    db_name,
    db_user,
    db_password,
    db_ssl_mode
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetDBCredentialsByUserID :many
SELECT * FROM db_credentials
WHERE user_id = $1;

-- name: GetDBCredentialByID :one
SELECT * FROM db_credentials
WHERE id = $1 LIMIT 1;

-- name: UpdateDBCredential :one
UPDATE db_credentials
SET
    db_host = $2,
    db_port = $3,
    db_name = $4,
    db_user = $5,
    db_password = $6,
    db_ssl_mode = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDBCredential :exec
DELETE FROM db_credentials
WHERE id = $1 AND user_id = $2;

-- name: CreateIndexer :one
INSERT INTO indexers (
    user_id,
    db_credential_id,
    indexer_type,
    params,
    target_table,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetIndexersByUserID :many
SELECT * FROM indexers
WHERE user_id = $1;

-- name: GetIndexerByID :one
SELECT * FROM indexers
WHERE id = $1 LIMIT 1;

-- name: GetIndexerByWebhookID :one
SELECT * FROM indexers
WHERE webhook_id = $1 LIMIT 1;

-- name: UpdateIndexerStatus :one
UPDATE indexers
SET
    status = $2,
    error_message = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateIndexerWebhookID :one
UPDATE indexers
SET
    webhook_id = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateLastIndexedTime :one
UPDATE indexers
SET
    last_indexed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteIndexer :exec
DELETE FROM indexers
WHERE id = $1 AND user_id = $2;

-- name: CreateIndexingLog :one
INSERT INTO indexing_logs (
    indexer_id,
    event_type,
    message,
    details
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetIndexingLogsByIndexerID :many
SELECT * FROM indexing_logs
WHERE indexer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetActiveIndexers :many
SELECT * FROM indexers
WHERE status = 'active';
