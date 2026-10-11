-- Create "extra_files" table
CREATE TABLE "extra_files" ("id" uuid NOT NULL, "kind" text NOT NULL, "path" text NOT NULL, "extension" text NOT NULL DEFAULT (''), "language" text NOT NULL DEFAULT (''), "language_tags" json NULL, "added" datetime NOT NULL, "last_updated" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "extra_files_items_extra_files" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "extrafile_item_id" to table: "extra_files"
CREATE INDEX "extrafile_item_id" ON "extra_files" ("item_id");
