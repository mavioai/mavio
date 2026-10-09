-- Create "plugin_configs" table
CREATE TABLE "plugin_configs" ("id" uuid NOT NULL, "plugin_id" character varying NOT NULL, "config" text NOT NULL, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "plugin_configs_plugin_id_key" to table: "plugin_configs"
CREATE UNIQUE INDEX "plugin_configs_plugin_id_key" ON "plugin_configs" ("plugin_id");
