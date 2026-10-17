-- Create "plugin_containers" table
CREATE TABLE "plugin_containers" ("id" uuid NOT NULL, "plugin_id" text NOT NULL, "name" text NOT NULL, "mode" text NOT NULL, "address" text NOT NULL DEFAULT (''), "secrets" json NULL, "use_gpu" bool NOT NULL DEFAULT (false), "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "plugincontainer_plugin_id_name" to table: "plugin_containers"
CREATE UNIQUE INDEX "plugincontainer_plugin_id_name" ON "plugin_containers" ("plugin_id", "name");
