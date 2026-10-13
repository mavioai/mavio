-- Modify "media_sources" table
ALTER TABLE "media_sources" ADD COLUMN "file_hash" character varying NOT NULL DEFAULT '';
