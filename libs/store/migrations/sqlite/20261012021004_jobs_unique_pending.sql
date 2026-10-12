-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_jobs" table
CREATE TABLE "new_jobs" ("id" uuid NOT NULL, "kind" text NOT NULL, "payload" blob NULL, "unique_key" text NOT NULL DEFAULT (''), "state" text NOT NULL, "priority" integer NOT NULL DEFAULT (0), "attempts" integer NOT NULL DEFAULT (0), "max_attempts" integer NOT NULL, "run_at" datetime NOT NULL, "lease_owner" text NOT NULL DEFAULT (''), "lease_expires_at" datetime NULL, "last_error" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "finished_at" datetime NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "jobs" to new temporary table "new_jobs"
INSERT INTO "new_jobs" ("id", "kind", "payload", "unique_key", "state", "priority", "attempts", "max_attempts", "run_at", "lease_owner", "lease_expires_at", "last_error", "created_at", "finished_at") SELECT "id", "kind", "payload", "unique_key", "state", "priority", "attempts", "max_attempts", "run_at", "lease_owner", "lease_expires_at", "last_error", "created_at", "finished_at" FROM "jobs";
-- Drop "jobs" table after copying rows
DROP TABLE "jobs";
-- Rename temporary table "new_jobs" to "jobs"
ALTER TABLE "new_jobs" RENAME TO "jobs";
-- Create index "job_unique_key" to table: "jobs"
CREATE UNIQUE INDEX "job_unique_key" ON "jobs" ("unique_key") WHERE unique_key <> '' AND state = 'pending';
-- Create index "job_unique_key_running" to table: "jobs"
CREATE INDEX "job_unique_key_running" ON "jobs" ("unique_key") WHERE unique_key <> '' AND state = 'running';
-- Create index "job_state_kind_priority_run_at" to table: "jobs"
CREATE INDEX "job_state_kind_priority_run_at" ON "jobs" ("state", "kind", "priority", "run_at");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
