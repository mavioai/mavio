-- Create "storage_jobs" table
CREATE TABLE "storage_jobs" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "key" character varying NOT NULL, "remote" character varying NOT NULL, "execute_id" character varying NULL, "job_id" bigint NULL, "data" jsonb NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "storagejob_kind_key" to table: "storage_jobs"
CREATE UNIQUE INDEX "storagejob_kind_key" ON "storage_jobs" ("kind", "key");
