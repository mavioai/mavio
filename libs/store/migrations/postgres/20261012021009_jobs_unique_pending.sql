-- Drop index "job_unique_key" from table: "jobs"
DROP INDEX "job_unique_key";
-- Create index "job_unique_key" to table: "jobs"
CREATE UNIQUE INDEX "job_unique_key" ON "jobs" ("unique_key") WHERE (((unique_key)::text <> ''::text) AND ((state)::text = 'pending'::text));
-- Create index "job_unique_key_running" to table: "jobs"
CREATE INDEX "job_unique_key_running" ON "jobs" ("unique_key") WHERE (((unique_key)::text <> ''::text) AND ((state)::text = 'running'::text));
