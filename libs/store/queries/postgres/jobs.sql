-- name: EnqueueJob :execrows
INSERT INTO jobs (id, kind, payload, unique_key, state, priority, attempts, max_attempts, run_at, lease_owner, last_error, created_at)
VALUES (@id, @kind, @payload, @unique_key, 'pending', @priority, 0, @max_attempts, @run_at, '', '', @created_at)
ON CONFLICT (unique_key) WHERE unique_key <> '' AND state IN ('pending', 'running') DO NOTHING;

-- name: LeaseJob :one
UPDATE jobs
SET state = 'running', attempts = attempts + 1, lease_owner = @owner, lease_expires_at = @expires_at
WHERE id = (
  SELECT j.id FROM jobs j
  -- Parameters in the order of the SQLite query, so that the generated
  -- parameter structs convert.
  WHERE ((j.state = 'pending' AND j.run_at <= @now) OR (j.state = 'running' AND j.lease_expires_at < @now))
    AND j.kind = ANY(@kinds::text[])
  ORDER BY j.priority DESC, j.run_at, j.id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: GetLeasedJob :one
SELECT * FROM jobs WHERE id = @id AND state = 'running' AND lease_owner = @owner;

-- name: ExtendJob :execrows
UPDATE jobs SET lease_expires_at = @expires_at
WHERE id = @id AND state = 'running' AND lease_owner = @owner;

-- name: FinishJob :execrows
UPDATE jobs
SET state = @state, finished_at = @finished_at, run_at = @run_at, last_error = @last_error,
    lease_owner = '', lease_expires_at = NULL
WHERE id = @id AND state = 'running' AND lease_owner = @owner;
