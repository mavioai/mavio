-- Create "item_localizations" table
CREATE TABLE "item_localizations" ("id" uuid NOT NULL, "language" character varying NOT NULL, "name" character varying NOT NULL DEFAULT '', "overview" text NOT NULL DEFAULT '', "tagline" character varying NOT NULL DEFAULT '', "search_key" character varying NOT NULL DEFAULT '', "sort_key" character varying NOT NULL DEFAULT '', "fetched_at" timestamptz NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "item_localizations_items_localizations" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "itemlocalization_item_id_language" to table: "item_localizations"
CREATE UNIQUE INDEX "itemlocalization_item_id_language" ON "item_localizations" ("item_id", "language");
