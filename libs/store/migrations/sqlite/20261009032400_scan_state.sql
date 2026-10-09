-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_libraries" table
CREATE TABLE "new_libraries" ("id" uuid NOT NULL, "name" text NOT NULL, "kind" text NOT NULL, "paths" json NOT NULL, "scan_interval" integer NOT NULL DEFAULT (0), "preferred_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "scan_generation" integer NOT NULL DEFAULT (0), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "libraries" to new temporary table "new_libraries"
INSERT INTO "new_libraries" ("id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "created_at", "updated_at") SELECT "id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "created_at", "updated_at" FROM "libraries";
-- Drop "libraries" table after copying rows
DROP TABLE "libraries";
-- Rename temporary table "new_libraries" to "libraries"
ALTER TABLE "new_libraries" RENAME TO "libraries";
-- Create "new_media_sources" table
CREATE TABLE "new_media_sources" ("id" uuid NOT NULL, "ord" integer NOT NULL, "path" text NOT NULL, "parts" json NULL, "disc" text NOT NULL DEFAULT (''), "name" text NOT NULL DEFAULT (''), "container" text NOT NULL DEFAULT (''), "size" integer NOT NULL DEFAULT (0), "modified" datetime NULL, "duration" integer NOT NULL DEFAULT (0), "bitrate" integer NOT NULL DEFAULT (0), "streams" json NULL, "chapters" json NULL, "keyframes" json NULL, "probed_at" datetime NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Copy rows from old table "media_sources" to new temporary table "new_media_sources"
INSERT INTO "new_media_sources" ("id", "ord", "path", "name", "container", "size", "duration", "bitrate", "streams", "chapters", "keyframes", "probed_at", "item_id") SELECT "id", "ord", "path", "name", "container", "size", "duration", "bitrate", "streams", "chapters", "keyframes", "probed_at", "item_id" FROM "media_sources";
-- Drop "media_sources" table after copying rows
DROP TABLE "media_sources";
-- Rename temporary table "new_media_sources" to "media_sources"
ALTER TABLE "new_media_sources" RENAME TO "media_sources";
-- Create index "mediasource_item_id_ord" to table: "media_sources"
CREATE UNIQUE INDEX "mediasource_item_id_ord" ON "media_sources" ("item_id", "ord");
-- Create "new_items" table
CREATE TABLE "new_items" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL, "sort_name" text NOT NULL, "sort_key" text NOT NULL, "original_title" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "original_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "tagline" text NOT NULL DEFAULT (''), "path" text NOT NULL DEFAULT (''), "index_number" integer NULL, "parent_index_number" integer NULL, "index_number_end" integer NULL, "production_year" integer NOT NULL DEFAULT (0), "premiere_date" datetime NULL, "end_date" datetime NULL, "runtime" integer NOT NULL DEFAULT (0), "official_rating" text NOT NULL DEFAULT (''), "custom_rating" text NOT NULL DEFAULT (''), "parental_rating" integer NOT NULL DEFAULT (0), "community_rating" real NOT NULL DEFAULT (0), "critic_rating" real NOT NULL DEFAULT (0), "external_ids" json NULL, "production_locations" json NULL, "remote_trailers" json NULL, "collection_name" text NOT NULL DEFAULT (''), "aspect_ratio" text NOT NULL DEFAULT (''), "video_3d_format" text NOT NULL DEFAULT (''), "album" text NOT NULL DEFAULT (''), "series_status" text NOT NULL DEFAULT (''), "air_days" json NULL, "air_time" text NOT NULL DEFAULT (''), "display_order" text NOT NULL DEFAULT (''), "airs_before_season_number" integer NULL, "airs_after_season_number" integer NULL, "airs_before_episode_number" integer NULL, "metadata_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "locked" bool NOT NULL DEFAULT (false), "locked_fields" json NULL, "extra" text NOT NULL DEFAULT (''), "date_added" datetime NOT NULL, "file_modified" datetime NULL, "metadata_refreshed_at" datetime NULL, "scan_generation" integer NOT NULL DEFAULT (0), "missing_since" datetime NULL, "parent_id" uuid NULL, "owner_id" uuid NULL, "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "items_items_children" FOREIGN KEY ("parent_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_items_extras" FOREIGN KEY ("owner_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE);
-- Copy rows from old table "items" to new temporary table "new_items"
INSERT INTO "new_items" ("id", "kind", "name", "sort_name", "sort_key", "original_title", "search_key", "original_key", "overview", "tagline", "path", "index_number", "parent_index_number", "index_number_end", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "custom_rating", "parental_rating", "community_rating", "critic_rating", "external_ids", "production_locations", "remote_trailers", "collection_name", "aspect_ratio", "video_3d_format", "album", "series_status", "air_days", "air_time", "display_order", "airs_before_season_number", "airs_after_season_number", "airs_before_episode_number", "metadata_language", "metadata_country", "locked", "locked_fields", "extra", "date_added", "file_modified", "metadata_refreshed_at", "parent_id", "owner_id", "library_id") SELECT "id", "kind", "name", "sort_name", "sort_key", "original_title", "search_key", "original_key", "overview", "tagline", "path", "index_number", "parent_index_number", "index_number_end", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "custom_rating", "parental_rating", "community_rating", "critic_rating", "external_ids", "production_locations", "remote_trailers", "collection_name", "aspect_ratio", "video_3d_format", "album", "series_status", "air_days", "air_time", "display_order", "airs_before_season_number", "airs_after_season_number", "airs_before_episode_number", "metadata_language", "metadata_country", "locked", "locked_fields", "extra", "date_added", "file_modified", "metadata_refreshed_at", "parent_id", "owner_id", "library_id" FROM "items";
-- Drop "items" table after copying rows
DROP TABLE "items";
-- Rename temporary table "new_items" to "items"
ALTER TABLE "new_items" RENAME TO "items";
-- Create index "item_library_id_path" to table: "items"
CREATE UNIQUE INDEX "item_library_id_path" ON "items" ("library_id", "path") WHERE path <> '';
-- Create index "item_parent_id_sort_key" to table: "items"
CREATE INDEX "item_parent_id_sort_key" ON "items" ("parent_id", "sort_key");
-- Create index "item_library_id_kind_sort_key" to table: "items"
CREATE INDEX "item_library_id_kind_sort_key" ON "items" ("library_id", "kind", "sort_key");
-- Create index "item_owner_id" to table: "items"
CREATE INDEX "item_owner_id" ON "items" ("owner_id");
-- Create index "item_date_added" to table: "items"
CREATE INDEX "item_date_added" ON "items" ("date_added");
-- Create index "item_library_id_scan_generation" to table: "items"
CREATE INDEX "item_library_id_scan_generation" ON "items" ("library_id", "scan_generation");
-- Create "folder_states" table
CREATE TABLE "folder_states" ("id" uuid NOT NULL, "path" text NOT NULL, "mod_time" datetime NOT NULL, "file_id" text NOT NULL DEFAULT (''), "entries" json NULL, "generation" integer NOT NULL DEFAULT (0), "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "folder_states_libraries_folder_states" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE);
-- Create index "folderstate_library_id_path" to table: "folder_states"
CREATE UNIQUE INDEX "folderstate_library_id_path" ON "folder_states" ("library_id", "path");
-- Create index "folderstate_library_id_generation" to table: "folder_states"
CREATE INDEX "folderstate_library_id_generation" ON "folder_states" ("library_id", "generation");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
