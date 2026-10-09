-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Add column "loudness" to table: "items"
ALTER TABLE "items" ADD COLUMN "loudness" real NULL;
-- Create "new_libraries" table
CREATE TABLE "new_libraries" ("id" uuid NOT NULL, "name" text NOT NULL, "kind" text NOT NULL, "paths" json NOT NULL, "scan_interval" integer NOT NULL DEFAULT (0), "preferred_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "save_local_metadata" bool NOT NULL DEFAULT (false), "auto_collections" bool NOT NULL DEFAULT (false), "extract_trickplay" bool NOT NULL DEFAULT (false), "extract_chapter_images" bool NOT NULL DEFAULT (false), "analyze_loudness" bool NOT NULL DEFAULT (false), "scan_generation" integer NOT NULL DEFAULT (0), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "libraries" to new temporary table "new_libraries"
INSERT INTO "new_libraries" ("id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "save_local_metadata", "auto_collections", "scan_generation", "created_at", "updated_at") SELECT "id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "save_local_metadata", "auto_collections", "scan_generation", "created_at", "updated_at" FROM "libraries";
-- Drop "libraries" table after copying rows
DROP TABLE "libraries";
-- Rename temporary table "new_libraries" to "libraries"
ALTER TABLE "new_libraries" RENAME TO "libraries";
-- Create index "library_kind" to table: "libraries"
CREATE UNIQUE INDEX "library_kind" ON "libraries" ("kind") WHERE kind IN ('collections', 'playlists');
-- Create "media_segments" table
CREATE TABLE "media_segments" ("id" uuid NOT NULL, "kind" text NOT NULL, "start" integer NOT NULL, "end" integer NOT NULL, "provider" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_segments_items_segments" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "mediasegment_item_id_start" to table: "media_segments"
CREATE INDEX "mediasegment_item_id_start" ON "media_segments" ("item_id", "start");
-- Create "trickplay" table
CREATE TABLE "trickplay" ("id" uuid NOT NULL, "width" integer NOT NULL, "height" integer NOT NULL, "tile_width" integer NOT NULL, "tile_height" integer NOT NULL, "thumbnail_count" integer NOT NULL, "interval" integer NOT NULL, "bandwidth" integer NOT NULL DEFAULT (0), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "trickplay_items_trickplay" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "trickplay_item_id_width" to table: "trickplay"
CREATE UNIQUE INDEX "trickplay_item_id_width" ON "trickplay" ("item_id", "width");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
