-- Drop index "itemvalue_kind_value" from table: "item_values"
DROP INDEX "itemvalue_kind_value";
-- Modify "item_values" table
ALTER TABLE "item_values" ADD COLUMN "value_key" character varying NOT NULL DEFAULT '', ADD COLUMN "sort_key" character varying NOT NULL DEFAULT '';
-- Create index "itemvalue_kind_value_key" to table: "item_values"
CREATE INDEX "itemvalue_kind_value_key" ON "item_values" ("kind", "value_key");
-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "original_key" character varying NOT NULL DEFAULT '';
-- Modify "people" table
ALTER TABLE "people" ADD COLUMN "search_key" character varying NOT NULL DEFAULT '', ADD COLUMN "sort_key" character varying NOT NULL DEFAULT '';
