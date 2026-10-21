-- Create "download_path_mappings" table
CREATE TABLE "download_path_mappings" ("id" uuid NOT NULL, "plugin_id" character varying NOT NULL, "client_id" character varying NOT NULL, "remote_path" character varying NOT NULL, "local_path" character varying NOT NULL, PRIMARY KEY ("id"));
-- Create index "downloadpathmapping_plugin_id_client_id_remote_path" to table: "download_path_mappings"
CREATE UNIQUE INDEX "downloadpathmapping_plugin_id_client_id_remote_path" ON "download_path_mappings" ("plugin_id", "client_id", "remote_path");
-- Drop "remote_path_mappings" table, replaced by "download_path_mappings"
DROP TABLE "remote_path_mappings";
