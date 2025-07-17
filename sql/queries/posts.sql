-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: GetAllPosts :many
SELECT * FROM posts;

-- name: GetPostFromURL :one
SELECT * FROM posts
WHERE url = $1;

-- name: GetPostsFromFeed :many
SELECT * FROM posts
WHERE feed_id = $1;

-- name: GetPostsForUser :many
SELECT title, url, description, published_at FROM posts WHERE feed_id IN 
    (SELECT feed_id FROM feed_follows WHERE user_id = $1)
ORDER BY published_at DESC NULLS LAST
LIMIT $2;

-- name: ResetPosts :exec
DELETE FROM posts;