-- Create "extra_files" table
CREATE TABLE "extra_files" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "path" text NOT NULL, "extension" character varying NOT NULL DEFAULT '', "language" character varying NOT NULL DEFAULT '', "language_tags" jsonb NULL, "added" timestamptz NOT NULL, "last_updated" timestamptz NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "extra_files_items_extra_files" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "extrafile_item_id" to table: "extra_files"
CREATE INDEX "extrafile_item_id" ON "extra_files" ("item_id");
