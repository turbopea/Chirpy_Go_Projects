-- name: CreateMessage :one
INSERT INTO chirps (message_id, created_at, updated_at, body, user_id)
VALUES (gen_random_uuid(), NOW(), NOW(),$1, $2)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetSingleChirp :one
SELECT * FROM chirps
WHERE message_id = $1;

-- name: DelChirp :exec
DELETE FROM chirps
WHERE message_id = $1;

-- name: GetAllUserChirps :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetAllUserChirpsDESC :many
SELECT * FROM chirps 
ORDER BY created_at DESC;

-- name: GetAllUserChirpsByID :many
SELECT * FROM chirps
WHERE user_id = $1
ORDER BY created_at ASC;
