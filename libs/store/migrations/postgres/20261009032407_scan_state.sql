-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "scan_generation" bigint NOT NULL DEFAULT 0, ADD COLUMN "missing_since" timestamptz NULL;
-- Create index "item_library_id_scan_generation" to table: "items"
CREATE INDEX "item_library_id_scan_generation" ON "items" ("library_id", "scan_generation");
-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN "scan_generation" bigint NOT NULL DEFAULT 0;
-- Modify "media_sources" table
ALTER TABLE "media_sources" ADD COLUMN "parts" jsonb NULL, ADD COLUMN "disc" character varying NOT NULL DEFAULT '', ADD COLUMN "modified" timestamptz NULL;
-- Create "folder_states" table
CREATE TABLE "folder_states" ("id" uuid NOT NULL, "path" character varying NOT NULL, "mod_time" timestamptz NOT NULL, "file_id" character varying NOT NULL DEFAULT '', "entries" jsonb NULL, "generation" bigint NOT NULL DEFAULT 0, "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "folder_states_libraries_folder_states" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "folderstate_library_id_generation" to table: "folder_states"
CREATE INDEX "folderstate_library_id_generation" ON "folder_states" ("library_id", "generation");
-- Create index "folderstate_library_id_path" to table: "folder_states"
CREATE UNIQUE INDEX "folderstate_library_id_path" ON "folder_states" ("library_id", "path");
