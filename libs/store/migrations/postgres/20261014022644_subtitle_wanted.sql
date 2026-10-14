-- Create "subtitle_wanted" table
CREATE TABLE "subtitle_wanted" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "library_id" uuid NOT NULL, "parent_id" uuid NULL, "path" text NOT NULL, "missing" jsonb NOT NULL, "updated" timestamptz NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_wanted_items_subtitle_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "subtitle_wanted_item_id_key" to table: "subtitle_wanted"
CREATE UNIQUE INDEX "subtitle_wanted_item_id_key" ON "subtitle_wanted" ("item_id");
-- Create index "subtitlewanted_kind_library_id_path" to table: "subtitle_wanted"
CREATE INDEX "subtitlewanted_kind_library_id_path" ON "subtitle_wanted" ("kind", "library_id", "path");
