-- Create "custom_formats" table
CREATE TABLE "custom_formats" ("id" uuid NOT NULL, "name" character varying NOT NULL, "include_when_renaming" boolean NOT NULL DEFAULT false, "specifications" jsonb NULL, PRIMARY KEY ("id"));
-- Create index "custom_formats_name_key" to table: "custom_formats"
CREATE UNIQUE INDEX "custom_formats_name_key" ON "custom_formats" ("name");
-- Create "delay_profiles" table
CREATE TABLE "delay_profiles" ("id" uuid NOT NULL, "is_default" boolean NOT NULL DEFAULT false, "ord" bigint NOT NULL DEFAULT 0, "enable_usenet" boolean NOT NULL DEFAULT false, "enable_torrent" boolean NOT NULL DEFAULT false, "preferred_protocol" character varying NOT NULL, "usenet_delay" bigint NOT NULL DEFAULT 0, "torrent_delay" bigint NOT NULL DEFAULT 0, "bypass_if_highest_quality" boolean NOT NULL DEFAULT false, "bypass_if_above_custom_format_score" boolean NOT NULL DEFAULT false, "minimum_custom_format_score" bigint NOT NULL DEFAULT 0, "tags" jsonb NULL, PRIMARY KEY ("id"));
-- Create index "delayprofile_is_default" to table: "delay_profiles"
CREATE UNIQUE INDEX "delayprofile_is_default" ON "delay_profiles" ("is_default") WHERE is_default;
-- Create index "delayprofile_ord" to table: "delay_profiles"
CREATE INDEX "delayprofile_ord" ON "delay_profiles" ("ord");
-- Create "quality_definitions" table
CREATE TABLE "quality_definitions" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "quality" bigint NOT NULL, "title" character varying NOT NULL, "min_size" double precision NOT NULL DEFAULT 0, "max_size" double precision NOT NULL DEFAULT 0, "preferred_size" double precision NOT NULL DEFAULT 0, PRIMARY KEY ("id"));
-- Create index "qualitydefinition_kind_quality" to table: "quality_definitions"
CREATE UNIQUE INDEX "qualitydefinition_kind_quality" ON "quality_definitions" ("kind", "quality");
-- Create "release_profiles" table
CREATE TABLE "release_profiles" ("id" uuid NOT NULL, "name" character varying NOT NULL DEFAULT '', "enabled" boolean NOT NULL DEFAULT false, "required" jsonb NULL, "ignored" jsonb NULL, "indexer_id" character varying NOT NULL DEFAULT '', "tags" jsonb NULL, PRIMARY KEY ("id"));
-- Create "remote_path_mappings" table
CREATE TABLE "remote_path_mappings" ("id" uuid NOT NULL, "host" character varying NOT NULL, "remote_path" character varying NOT NULL, "local_path" character varying NOT NULL, PRIMARY KEY ("id"));
-- Create index "remotepathmapping_host_remote_path" to table: "remote_path_mappings"
CREATE UNIQUE INDEX "remotepathmapping_host_remote_path" ON "remote_path_mappings" ("host", "remote_path");
-- Create "blocklist" table
CREATE TABLE "blocklist" ("id" uuid NOT NULL, "item_ids" jsonb NULL, "source_title" character varying NOT NULL, "quality" jsonb NOT NULL, "languages" jsonb NULL, "date" timestamptz NOT NULL, "published_date" timestamptz NULL, "size" bigint NOT NULL DEFAULT 0, "protocol" character varying NOT NULL, "indexer_id" character varying NOT NULL DEFAULT '', "indexer" character varying NOT NULL DEFAULT '', "indexer_flags" jsonb NULL, "message" text NOT NULL DEFAULT '', "torrent_info_hash" character varying NOT NULL DEFAULT '', "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "blocklist_items_blocklist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "blocklistentry_date" to table: "blocklist"
CREATE INDEX "blocklistentry_date" ON "blocklist" ("date");
-- Create index "blocklistentry_item_id_date" to table: "blocklist"
CREATE INDEX "blocklistentry_item_id_date" ON "blocklist" ("item_id", "date");
-- Create "downloads" table
CREATE TABLE "downloads" ("id" uuid NOT NULL, "plugin_id" character varying NOT NULL, "client_id" character varying NOT NULL, "download_id" character varying NOT NULL, "item_ids" jsonb NULL, "title" character varying NOT NULL DEFAULT '', "indexer_id" character varying NOT NULL DEFAULT '', "indexer" character varying NOT NULL DEFAULT '', "protocol" character varying NOT NULL, "quality" jsonb NOT NULL, "languages" jsonb NULL, "size" bigint NOT NULL DEFAULT 0, "state" character varying NOT NULL, "messages" jsonb NULL, "grabbed_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "downloads_items_downloads" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "download_grabbed_at" to table: "downloads"
CREATE INDEX "download_grabbed_at" ON "downloads" ("grabbed_at");
-- Create index "download_item_id" to table: "downloads"
CREATE INDEX "download_item_id" ON "downloads" ("item_id");
-- Create index "download_plugin_id_client_id_download_id" to table: "downloads"
CREATE UNIQUE INDEX "download_plugin_id_client_id_download_id" ON "downloads" ("plugin_id", "client_id", "download_id");
-- Create "history_events" table
CREATE TABLE "history_events" ("id" uuid NOT NULL, "type" character varying NOT NULL, "parent_id" uuid NULL, "date" timestamptz NOT NULL, "source_title" character varying NOT NULL DEFAULT '', "quality" jsonb NOT NULL, "languages" jsonb NULL, "download_id" character varying NOT NULL DEFAULT '', "data" jsonb NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "history_events_items_history" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "historyevent_date" to table: "history_events"
CREATE INDEX "historyevent_date" ON "history_events" ("date");
-- Create index "historyevent_download_id" to table: "history_events"
CREATE INDEX "historyevent_download_id" ON "history_events" ("download_id");
-- Create index "historyevent_item_id_date" to table: "history_events"
CREATE INDEX "historyevent_item_id_date" ON "history_events" ("item_id", "date");
-- Create index "historyevent_parent_id_date" to table: "history_events"
CREATE INDEX "historyevent_parent_id_date" ON "history_events" ("parent_id", "date");
-- Create "pending_releases" table
CREATE TABLE "pending_releases" ("id" uuid NOT NULL, "item_ids" jsonb NULL, "title" character varying NOT NULL DEFAULT '', "added" timestamptz NOT NULL, "reason" character varying NOT NULL, "release" text NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "pending_releases_items_pending_releases" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "pendingrelease_item_id_added" to table: "pending_releases"
CREATE INDEX "pendingrelease_item_id_added" ON "pending_releases" ("item_id", "added");
-- Create "quality_profiles" table
CREATE TABLE "quality_profiles" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "name" character varying NOT NULL, "upgrade_allowed" boolean NOT NULL DEFAULT false, "cutoff" bigint NOT NULL DEFAULT 0, "items" jsonb NOT NULL, "min_format_score" bigint NOT NULL DEFAULT 0, "cutoff_format_score" bigint NOT NULL DEFAULT 0, "min_upgrade_format_score" bigint NOT NULL DEFAULT 0, "format_scores" jsonb NULL, "language" character varying NOT NULL DEFAULT '', PRIMARY KEY ("id"));
-- Create index "qualityprofile_kind_name" to table: "quality_profiles"
CREATE UNIQUE INDEX "qualityprofile_kind_name" ON "quality_profiles" ("kind", "name");
-- Create "wanted" table
CREATE TABLE "wanted" ("id" uuid NOT NULL, "minimum_availability" character varying NOT NULL DEFAULT '', "tags" jsonb NULL, "added_at" timestamptz NOT NULL, "last_searched_at" timestamptz NULL, "item_id" uuid NOT NULL, "quality_profile_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "wanted_items_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "wanted_quality_profiles_wanted" FOREIGN KEY ("quality_profile_id") REFERENCES "quality_profiles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "wanted_item_id_key" to table: "wanted"
CREATE UNIQUE INDEX "wanted_item_id_key" ON "wanted" ("item_id");
