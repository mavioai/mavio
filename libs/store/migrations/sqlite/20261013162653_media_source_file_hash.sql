-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_media_sources" table
CREATE TABLE "new_media_sources" ("id" uuid NOT NULL, "ord" integer NOT NULL, "path" text NOT NULL, "parts" json NULL, "disc" text NOT NULL DEFAULT (''), "name" text NOT NULL DEFAULT (''), "container" text NOT NULL DEFAULT (''), "size" integer NOT NULL DEFAULT (0), "modified" datetime NULL, "duration" integer NOT NULL DEFAULT (0), "bitrate" integer NOT NULL DEFAULT (0), "streams" json NULL, "chapters" json NULL, "keyframes" json NULL, "probed_at" datetime NULL, "file_hash" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Copy rows from old table "media_sources" to new temporary table "new_media_sources"
INSERT INTO "new_media_sources" ("id", "ord", "path", "parts", "disc", "name", "container", "size", "modified", "duration", "bitrate", "streams", "chapters", "keyframes", "probed_at", "item_id") SELECT "id", "ord", "path", "parts", "disc", "name", "container", "size", "modified", "duration", "bitrate", "streams", "chapters", "keyframes", "probed_at", "item_id" FROM "media_sources";
-- Drop "media_sources" table after copying rows
DROP TABLE "media_sources";
-- Rename temporary table "new_media_sources" to "media_sources"
ALTER TABLE "new_media_sources" RENAME TO "media_sources";
-- Create index "mediasource_item_id_ord" to table: "media_sources"
CREATE UNIQUE INDEX "mediasource_item_id_ord" ON "media_sources" ("item_id", "ord");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
