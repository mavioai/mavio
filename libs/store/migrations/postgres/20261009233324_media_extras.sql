-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "loudness" double precision NULL;
-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN "extract_trickplay" boolean NOT NULL DEFAULT false, ADD COLUMN "extract_chapter_images" boolean NOT NULL DEFAULT false, ADD COLUMN "analyze_loudness" boolean NOT NULL DEFAULT false;
-- Create "media_segments" table
CREATE TABLE "media_segments" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "start" bigint NOT NULL, "end" bigint NOT NULL, "provider" character varying NOT NULL DEFAULT '', "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_segments_items_segments" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "mediasegment_item_id_start" to table: "media_segments"
CREATE INDEX "mediasegment_item_id_start" ON "media_segments" ("item_id", "start");
-- Create "trickplay" table
CREATE TABLE "trickplay" ("id" uuid NOT NULL, "width" bigint NOT NULL, "height" bigint NOT NULL, "tile_width" bigint NOT NULL, "tile_height" bigint NOT NULL, "thumbnail_count" bigint NOT NULL, "interval" bigint NOT NULL, "bandwidth" bigint NOT NULL DEFAULT 0, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "trickplay_items_trickplay" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "trickplay_item_id_width" to table: "trickplay"
CREATE UNIQUE INDEX "trickplay_item_id_width" ON "trickplay" ("item_id", "width");
