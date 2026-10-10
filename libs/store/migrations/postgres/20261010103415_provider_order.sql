-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN "download_lyrics" boolean NOT NULL DEFAULT false, ADD COLUMN "provider_order" jsonb NULL;
