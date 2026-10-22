-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_plugin_containers" table
CREATE TABLE "new_plugin_containers" ("id" uuid NOT NULL, "plugin_id" text NOT NULL, "name" text NOT NULL, "mode" text NOT NULL, "address" text NOT NULL DEFAULT (''), "secrets" json NULL, "use_gpu" bool NOT NULL DEFAULT (false), "proxy" text NOT NULL DEFAULT ('system'), "folders" json NULL, "peer_port" integer NOT NULL DEFAULT (0), "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "plugin_containers" to new temporary table "new_plugin_containers"
INSERT INTO "new_plugin_containers" ("id", "plugin_id", "name", "mode", "address", "secrets", "use_gpu", "proxy", "updated_at") SELECT "id", "plugin_id", "name", "mode", "address", "secrets", "use_gpu", "proxy", "updated_at" FROM "plugin_containers";
-- Drop "plugin_containers" table after copying rows
DROP TABLE "plugin_containers";
-- Rename temporary table "new_plugin_containers" to "plugin_containers"
ALTER TABLE "new_plugin_containers" RENAME TO "plugin_containers";
-- Create index "plugincontainer_plugin_id_name" to table: "plugin_containers"
CREATE UNIQUE INDEX "plugincontainer_plugin_id_name" ON "plugin_containers" ("plugin_id", "name");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
