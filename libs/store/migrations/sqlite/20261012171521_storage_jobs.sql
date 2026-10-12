-- Create "storage_jobs" table
CREATE TABLE "storage_jobs" ("id" uuid NOT NULL, "kind" text NOT NULL, "key" text NOT NULL, "remote" text NOT NULL, "execute_id" text NULL, "job_id" integer NULL, "data" json NULL, "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "storagejob_kind_key" to table: "storage_jobs"
CREATE UNIQUE INDEX "storagejob_kind_key" ON "storage_jobs" ("kind", "key");
