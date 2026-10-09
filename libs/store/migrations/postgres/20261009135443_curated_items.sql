-- Create index "library_kind" to table: "libraries"
CREATE UNIQUE INDEX "library_kind" ON "libraries" ("kind") WHERE ((kind)::text = ANY ((ARRAY['collections'::character varying, 'playlists'::character varying])::text[]));
-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "user_id" uuid NULL, ADD CONSTRAINT "items_users_playlists" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create "item_links" table
CREATE TABLE "item_links" ("id" uuid NOT NULL, "ord" bigint NOT NULL, "container_id" uuid NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "item_links_items_linked_in" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "item_links_items_links" FOREIGN KEY ("container_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "itemlink_container_id_ord" to table: "item_links"
CREATE INDEX "itemlink_container_id_ord" ON "item_links" ("container_id", "ord");
-- Create index "itemlink_item_id" to table: "item_links"
CREATE INDEX "itemlink_item_id" ON "item_links" ("item_id");
