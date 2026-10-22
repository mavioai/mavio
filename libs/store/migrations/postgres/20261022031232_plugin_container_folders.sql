-- Modify "plugin_containers" table
ALTER TABLE "plugin_containers" ADD COLUMN "folders" jsonb NULL, ADD COLUMN "peer_port" bigint NOT NULL DEFAULT 0;
