-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_items" table
CREATE TABLE "new_items" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL, "sort_name" text NOT NULL, "sort_key" text NOT NULL, "original_title" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "original_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "tagline" text NOT NULL DEFAULT (''), "path" text NOT NULL DEFAULT (''), "index_number" integer NULL, "parent_index_number" integer NULL, "index_number_end" integer NULL, "production_year" integer NOT NULL DEFAULT (0), "premiere_date" datetime NULL, "end_date" datetime NULL, "runtime" integer NOT NULL DEFAULT (0), "official_rating" text NOT NULL DEFAULT (''), "parental_rating" integer NOT NULL DEFAULT (0), "community_rating" real NOT NULL DEFAULT (0), "critic_rating" real NOT NULL DEFAULT (0), "external_ids" json NULL, "series_status" text NOT NULL DEFAULT (''), "extra" text NOT NULL DEFAULT (''), "date_added" datetime NOT NULL, "file_modified" datetime NULL, "metadata_refreshed_at" datetime NULL, "parent_id" uuid NULL, "owner_id" uuid NULL, "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "items_items_children" FOREIGN KEY ("parent_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_items_extras" FOREIGN KEY ("owner_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE);
-- Copy rows from old table "items" to new temporary table "new_items"
INSERT INTO "new_items" ("id", "kind", "name", "sort_name", "sort_key", "original_title", "search_key", "overview", "tagline", "path", "index_number", "parent_index_number", "index_number_end", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "parental_rating", "community_rating", "critic_rating", "external_ids", "series_status", "extra", "date_added", "file_modified", "metadata_refreshed_at", "parent_id", "owner_id", "library_id") SELECT "id", "kind", "name", "sort_name", "sort_key", "original_title", "search_key", "overview", "tagline", "path", "index_number", "parent_index_number", "index_number_end", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "parental_rating", "community_rating", "critic_rating", "external_ids", "series_status", "extra", "date_added", "file_modified", "metadata_refreshed_at", "parent_id", "owner_id", "library_id" FROM "items";
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
-- Create "new_item_values" table
CREATE TABLE "new_item_values" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "kind" text NOT NULL, "value" text NOT NULL, "value_key" text NOT NULL DEFAULT (''), "sort_key" text NOT NULL DEFAULT (''), "ord" integer NOT NULL, "item_id" uuid NOT NULL, CONSTRAINT "item_values_items_values" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Copy rows from old table "item_values" to new temporary table "new_item_values"
INSERT INTO "new_item_values" ("id", "kind", "value", "ord", "item_id") SELECT "id", "kind", "value", "ord", "item_id" FROM "item_values";
-- Drop "item_values" table after copying rows
DROP TABLE "item_values";
-- Rename temporary table "new_item_values" to "item_values"
ALTER TABLE "new_item_values" RENAME TO "item_values";
-- Create index "itemvalue_item_id_kind_ord" to table: "item_values"
CREATE UNIQUE INDEX "itemvalue_item_id_kind_ord" ON "item_values" ("item_id", "kind", "ord");
-- Create index "itemvalue_kind_value_key" to table: "item_values"
CREATE INDEX "itemvalue_kind_value_key" ON "item_values" ("kind", "value_key");
-- Create "new_people" table
CREATE TABLE "new_people" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "sort_name" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "sort_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "birth_date" datetime NULL, "death_date" datetime NULL, "birth_place" text NOT NULL DEFAULT (''), "external_ids" json NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "people" to new temporary table "new_people"
INSERT INTO "new_people" ("id", "name", "name_key", "sort_name", "overview", "birth_date", "death_date", "birth_place", "external_ids") SELECT "id", "name", "name_key", "sort_name", "overview", "birth_date", "death_date", "birth_place", "external_ids" FROM "people";
-- Drop "people" table after copying rows
DROP TABLE "people";
-- Rename temporary table "new_people" to "people"
ALTER TABLE "new_people" RENAME TO "people";
-- Create index "person_name_key" to table: "people"
CREATE INDEX "person_name_key" ON "people" ("name_key");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
