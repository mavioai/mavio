-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_libraries" table
CREATE TABLE "new_libraries" ("id" uuid NOT NULL, "name" text NOT NULL, "kind" text NOT NULL, "paths" json NOT NULL, "scan_interval" integer NOT NULL DEFAULT (0), "preferred_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "save_local_metadata" bool NOT NULL DEFAULT (false), "auto_collections" bool NOT NULL DEFAULT (false), "scan_generation" integer NOT NULL DEFAULT (0), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "libraries" to new temporary table "new_libraries"
INSERT INTO "new_libraries" ("id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "scan_generation", "created_at", "updated_at") SELECT "id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "scan_generation", "created_at", "updated_at" FROM "libraries";
-- Drop "libraries" table after copying rows
DROP TABLE "libraries";
-- Rename temporary table "new_libraries" to "libraries"
ALTER TABLE "new_libraries" RENAME TO "libraries";
-- Create index "library_kind" to table: "libraries"
CREATE UNIQUE INDEX "library_kind" ON "libraries" ("kind") WHERE kind IN ('collections', 'playlists');
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
