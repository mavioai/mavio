-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_wanted" table
CREATE TABLE "new_wanted" ("id" uuid NOT NULL, "minimum_availability" text NOT NULL DEFAULT (''), "monitor_new_items" text NOT NULL DEFAULT (''), "season_folder" bool NOT NULL DEFAULT (false), "tags" json NULL, "added_at" datetime NOT NULL, "last_searched_at" datetime NULL, "item_id" uuid NOT NULL, "quality_profile_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "wanted_items_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "wanted_quality_profiles_wanted" FOREIGN KEY ("quality_profile_id") REFERENCES "quality_profiles" ("id") ON DELETE NO ACTION);
-- Copy rows from old table "wanted" to new temporary table "new_wanted"
INSERT INTO "new_wanted" ("id", "minimum_availability", "tags", "added_at", "last_searched_at", "item_id", "quality_profile_id") SELECT "id", "minimum_availability", "tags", "added_at", "last_searched_at", "item_id", "quality_profile_id" FROM "wanted";
-- Drop "wanted" table after copying rows
DROP TABLE "wanted";
-- Rename temporary table "new_wanted" to "wanted"
ALTER TABLE "new_wanted" RENAME TO "wanted";
-- Create index "wanted_item_id_key" to table: "wanted"
CREATE UNIQUE INDEX "wanted_item_id_key" ON "wanted" ("item_id");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
