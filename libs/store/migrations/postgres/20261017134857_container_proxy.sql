-- Modify "plugin_containers" table
ALTER TABLE "plugin_containers" ADD COLUMN "proxy" character varying NOT NULL DEFAULT 'system';
