-- Create "blocklist" table
CREATE TABLE "blocklist" ("id" uuid NOT NULL, "item_ids" json NULL, "source_title" text NOT NULL, "quality" json NOT NULL, "languages" json NULL, "date" datetime NOT NULL, "published_date" datetime NULL, "size" integer NOT NULL DEFAULT (0), "protocol" text NOT NULL, "indexer_id" text NOT NULL DEFAULT (''), "indexer" text NOT NULL DEFAULT (''), "indexer_flags" json NULL, "message" text NOT NULL DEFAULT (''), "torrent_info_hash" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "blocklist_items_blocklist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "blocklistentry_item_id_date" to table: "blocklist"
CREATE INDEX "blocklistentry_item_id_date" ON "blocklist" ("item_id", "date");
-- Create index "blocklistentry_date" to table: "blocklist"
CREATE INDEX "blocklistentry_date" ON "blocklist" ("date");
-- Create "custom_formats" table
CREATE TABLE "custom_formats" ("id" uuid NOT NULL, "name" text NOT NULL, "include_when_renaming" bool NOT NULL DEFAULT (false), "specifications" json NULL, PRIMARY KEY ("id"));
-- Create index "custom_formats_name_key" to table: "custom_formats"
CREATE UNIQUE INDEX "custom_formats_name_key" ON "custom_formats" ("name");
-- Create "delay_profiles" table
CREATE TABLE "delay_profiles" ("id" uuid NOT NULL, "is_default" bool NOT NULL DEFAULT (false), "ord" integer NOT NULL DEFAULT (0), "enable_usenet" bool NOT NULL DEFAULT (false), "enable_torrent" bool NOT NULL DEFAULT (false), "preferred_protocol" text NOT NULL, "usenet_delay" integer NOT NULL DEFAULT (0), "torrent_delay" integer NOT NULL DEFAULT (0), "bypass_if_highest_quality" bool NOT NULL DEFAULT (false), "bypass_if_above_custom_format_score" bool NOT NULL DEFAULT (false), "minimum_custom_format_score" integer NOT NULL DEFAULT (0), "tags" json NULL, PRIMARY KEY ("id"));
-- Create index "delayprofile_is_default" to table: "delay_profiles"
CREATE UNIQUE INDEX "delayprofile_is_default" ON "delay_profiles" ("is_default") WHERE is_default;
-- Create index "delayprofile_ord" to table: "delay_profiles"
CREATE INDEX "delayprofile_ord" ON "delay_profiles" ("ord");
-- Create "downloads" table
CREATE TABLE "downloads" ("id" uuid NOT NULL, "plugin_id" text NOT NULL, "client_id" text NOT NULL, "download_id" text NOT NULL, "item_ids" json NULL, "title" text NOT NULL DEFAULT (''), "indexer_id" text NOT NULL DEFAULT (''), "indexer" text NOT NULL DEFAULT (''), "protocol" text NOT NULL, "quality" json NOT NULL, "languages" json NULL, "size" integer NOT NULL DEFAULT (0), "state" text NOT NULL, "messages" json NULL, "grabbed_at" datetime NOT NULL, "updated_at" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "downloads_items_downloads" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "download_plugin_id_client_id_download_id" to table: "downloads"
CREATE UNIQUE INDEX "download_plugin_id_client_id_download_id" ON "downloads" ("plugin_id", "client_id", "download_id");
-- Create index "download_item_id" to table: "downloads"
CREATE INDEX "download_item_id" ON "downloads" ("item_id");
-- Create index "download_grabbed_at" to table: "downloads"
CREATE INDEX "download_grabbed_at" ON "downloads" ("grabbed_at");
-- Create "history_events" table
CREATE TABLE "history_events" ("id" uuid NOT NULL, "type" text NOT NULL, "parent_id" uuid NULL, "date" datetime NOT NULL, "source_title" text NOT NULL DEFAULT (''), "quality" json NOT NULL, "languages" json NULL, "download_id" text NOT NULL DEFAULT (''), "data" json NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "history_events_items_history" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "historyevent_item_id_date" to table: "history_events"
CREATE INDEX "historyevent_item_id_date" ON "history_events" ("item_id", "date");
-- Create index "historyevent_parent_id_date" to table: "history_events"
CREATE INDEX "historyevent_parent_id_date" ON "history_events" ("parent_id", "date");
-- Create index "historyevent_download_id" to table: "history_events"
CREATE INDEX "historyevent_download_id" ON "history_events" ("download_id");
-- Create index "historyevent_date" to table: "history_events"
CREATE INDEX "historyevent_date" ON "history_events" ("date");
-- Create "pending_releases" table
CREATE TABLE "pending_releases" ("id" uuid NOT NULL, "item_ids" json NULL, "title" text NOT NULL DEFAULT (''), "added" datetime NOT NULL, "reason" text NOT NULL, "release" text NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "pending_releases_items_pending_releases" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "pendingrelease_item_id_added" to table: "pending_releases"
CREATE INDEX "pendingrelease_item_id_added" ON "pending_releases" ("item_id", "added");
-- Create "quality_definitions" table
CREATE TABLE "quality_definitions" ("id" uuid NOT NULL, "kind" text NOT NULL, "quality" integer NOT NULL, "title" text NOT NULL, "min_size" real NOT NULL DEFAULT (0), "max_size" real NOT NULL DEFAULT (0), "preferred_size" real NOT NULL DEFAULT (0), PRIMARY KEY ("id"));
-- Create index "qualitydefinition_kind_quality" to table: "quality_definitions"
CREATE UNIQUE INDEX "qualitydefinition_kind_quality" ON "quality_definitions" ("kind", "quality");
-- Create "quality_profiles" table
CREATE TABLE "quality_profiles" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL, "upgrade_allowed" bool NOT NULL DEFAULT (false), "cutoff" integer NOT NULL DEFAULT (0), "items" json NOT NULL, "min_format_score" integer NOT NULL DEFAULT (0), "cutoff_format_score" integer NOT NULL DEFAULT (0), "min_upgrade_format_score" integer NOT NULL DEFAULT (0), "format_scores" json NULL, "language" text NOT NULL DEFAULT (''), PRIMARY KEY ("id"));
-- Create index "qualityprofile_kind_name" to table: "quality_profiles"
CREATE UNIQUE INDEX "qualityprofile_kind_name" ON "quality_profiles" ("kind", "name");
-- Create "release_profiles" table
CREATE TABLE "release_profiles" ("id" uuid NOT NULL, "name" text NOT NULL DEFAULT (''), "enabled" bool NOT NULL DEFAULT (false), "required" json NULL, "ignored" json NULL, "indexer_id" text NOT NULL DEFAULT (''), "tags" json NULL, PRIMARY KEY ("id"));
-- Create "remote_path_mappings" table
CREATE TABLE "remote_path_mappings" ("id" uuid NOT NULL, "host" text NOT NULL, "remote_path" text NOT NULL, "local_path" text NOT NULL, PRIMARY KEY ("id"));
-- Create index "remotepathmapping_host_remote_path" to table: "remote_path_mappings"
CREATE UNIQUE INDEX "remotepathmapping_host_remote_path" ON "remote_path_mappings" ("host", "remote_path");
-- Create "wanted" table
CREATE TABLE "wanted" ("id" uuid NOT NULL, "minimum_availability" text NOT NULL DEFAULT (''), "tags" json NULL, "added_at" datetime NOT NULL, "last_searched_at" datetime NULL, "item_id" uuid NOT NULL, "quality_profile_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "wanted_items_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "wanted_quality_profiles_wanted" FOREIGN KEY ("quality_profile_id") REFERENCES "quality_profiles" ("id") ON DELETE NO ACTION);
-- Create index "wanted_item_id_key" to table: "wanted"
CREATE UNIQUE INDEX "wanted_item_id_key" ON "wanted" ("item_id");
