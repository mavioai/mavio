-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "virtual" boolean NOT NULL DEFAULT false, ADD COLUMN "monitored" boolean NOT NULL DEFAULT false, ADD COLUMN "series_type" character varying NOT NULL DEFAULT '', ADD COLUMN "absolute_number" bigint NULL;
-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN "virtual_episodes" jsonb NULL;
