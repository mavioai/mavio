-- Modify "wanted" table
ALTER TABLE "wanted" ADD COLUMN "monitor_new_items" character varying NOT NULL DEFAULT '', ADD COLUMN "season_folder" boolean NOT NULL DEFAULT false;
