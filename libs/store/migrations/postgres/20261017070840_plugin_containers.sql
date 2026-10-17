-- Create "plugin_containers" table
CREATE TABLE "plugin_containers" ("id" uuid NOT NULL, "plugin_id" character varying NOT NULL, "name" character varying NOT NULL, "mode" character varying NOT NULL, "address" character varying NOT NULL DEFAULT '', "secrets" jsonb NULL, "use_gpu" boolean NOT NULL DEFAULT false, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "plugincontainer_plugin_id_name" to table: "plugin_containers"
CREATE UNIQUE INDEX "plugincontainer_plugin_id_name" ON "plugin_containers" ("plugin_id", "name");
