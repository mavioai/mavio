-- Modify "blocked_titles" table
ALTER TABLE "blocked_titles" ADD COLUMN "tags" jsonb NULL;
