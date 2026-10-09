-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN "save_local_metadata" boolean NOT NULL DEFAULT false, ADD COLUMN "auto_collections" boolean NOT NULL DEFAULT false;
