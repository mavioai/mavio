-- Create "api_keys" table
CREATE TABLE "api_keys" ("id" uuid NOT NULL, "name" text NOT NULL, "token_hash" blob NOT NULL, "created_at" datetime NOT NULL, "last_used_at" datetime NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "api_keys_users_api_keys" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "api_keys_token_hash_key" to table: "api_keys"
CREATE UNIQUE INDEX "api_keys_token_hash_key" ON "api_keys" ("token_hash");
-- Create "activities" table
CREATE TABLE "activities" ("id" uuid NOT NULL, "time" datetime NOT NULL, "type" text NOT NULL, "severity" text NOT NULL, "title" text NOT NULL, "message" text NOT NULL DEFAULT (''), "user_id" uuid NULL, "item_id" uuid NULL, "attributes" json NULL, PRIMARY KEY ("id"));
-- Create index "activity_time" to table: "activities"
CREATE INDEX "activity_time" ON "activities" ("time");
-- Create "auth_sessions" table
CREATE TABLE "auth_sessions" ("id" uuid NOT NULL, "device_key" blob NOT NULL, "device_id" text NOT NULL, "device_name" text NOT NULL DEFAULT (''), "client" text NOT NULL DEFAULT (''), "client_version" text NOT NULL DEFAULT (''), "switcher_id" uuid NULL, "created_at" datetime NOT NULL, "last_seen_at" datetime NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_sessions_users_auth_sessions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "authsession_user_id_device_id" to table: "auth_sessions"
CREATE UNIQUE INDEX "authsession_user_id_device_id" ON "auth_sessions" ("user_id", "device_id");
-- Create "blocked_titles" table
CREATE TABLE "blocked_titles" ("id" uuid NOT NULL, "kind" text NOT NULL, "tmdb_id" integer NOT NULL, "title" text NOT NULL DEFAULT (''), "tags" json NULL, "created_at" datetime NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "blocked_titles_users_blocked_titles" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE SET NULL);
-- Create index "blockedtitle_kind_tmdb_id" to table: "blocked_titles"
CREATE UNIQUE INDEX "blockedtitle_kind_tmdb_id" ON "blocked_titles" ("kind", "tmdb_id");
-- Create index "blockedtitle_created_at" to table: "blocked_titles"
CREATE INDEX "blockedtitle_created_at" ON "blocked_titles" ("created_at");
-- Create "blocklist" table
CREATE TABLE "blocklist" ("id" uuid NOT NULL, "item_ids" json NULL, "source_title" text NOT NULL, "quality" json NOT NULL, "languages" json NULL, "date" datetime NOT NULL, "published_date" datetime NULL, "size" integer NOT NULL DEFAULT (0), "protocol" text NOT NULL, "indexer_id" text NOT NULL DEFAULT (''), "indexer" text NOT NULL DEFAULT (''), "indexer_flags" json NULL, "message" text NOT NULL DEFAULT (''), "torrent_info_hash" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "blocklist_items_blocklist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "blocklistentry_item_id_date" to table: "blocklist"
CREATE INDEX "blocklistentry_item_id_date" ON "blocklist" ("item_id", "date");
-- Create index "blocklistentry_date" to table: "blocklist"
CREATE INDEX "blocklistentry_date" ON "blocklist" ("date");
-- Create "credentials" table
CREATE TABLE "credentials" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL DEFAULT (''), "data" text NOT NULL, "created_at" datetime NOT NULL, "last_used_at" datetime NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "credentials_users_credentials" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "credential_user_id_kind" to table: "credentials"
CREATE UNIQUE INDEX "credential_user_id_kind" ON "credentials" ("user_id", "kind") WHERE kind <> 'passkey';
-- Create "credits" table
CREATE TABLE "credits" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "kind" text NOT NULL, "role" text NOT NULL DEFAULT (''), "ord" integer NOT NULL DEFAULT (0), "item_id" uuid NOT NULL, "person_id" uuid NOT NULL, CONSTRAINT "credits_items_credits" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "credits_people_credits" FOREIGN KEY ("person_id") REFERENCES "people" ("id") ON DELETE CASCADE);
-- Create index "credit_item_id_ord" to table: "credits"
CREATE INDEX "credit_item_id_ord" ON "credits" ("item_id", "ord");
-- Create index "credit_person_id" to table: "credits"
CREATE INDEX "credit_person_id" ON "credits" ("person_id");
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
-- Create "display_preferences" table
CREATE TABLE "display_preferences" ("id" uuid NOT NULL, "client" text NOT NULL, "view" text NOT NULL, "values" json NULL, "updated_at" datetime NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "display_preferences_users_display_preferences" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "displaypreferences_user_id_client_view" to table: "display_preferences"
CREATE UNIQUE INDEX "displaypreferences_user_id_client_view" ON "display_preferences" ("user_id", "client", "view");
-- Create "downloads" table
CREATE TABLE "downloads" ("id" uuid NOT NULL, "plugin_id" text NOT NULL, "client_id" text NOT NULL, "download_id" text NOT NULL, "item_ids" json NULL, "title" text NOT NULL DEFAULT (''), "indexer_id" text NOT NULL DEFAULT (''), "indexer" text NOT NULL DEFAULT (''), "protocol" text NOT NULL, "quality" json NOT NULL, "languages" json NULL, "size" integer NOT NULL DEFAULT (0), "state" text NOT NULL, "messages" json NULL, "grabbed_at" datetime NOT NULL, "updated_at" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "downloads_items_downloads" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "download_plugin_id_client_id_download_id" to table: "downloads"
CREATE UNIQUE INDEX "download_plugin_id_client_id_download_id" ON "downloads" ("plugin_id", "client_id", "download_id");
-- Create index "download_item_id" to table: "downloads"
CREATE INDEX "download_item_id" ON "downloads" ("item_id");
-- Create index "download_grabbed_at" to table: "downloads"
CREATE INDEX "download_grabbed_at" ON "downloads" ("grabbed_at");
-- Create "episode_states" table
CREATE TABLE "episode_states" ("id" uuid NOT NULL, "season" integer NOT NULL, "episode" integer NOT NULL, "scene_season" integer NULL, "scene_episode" integer NULL, "scene_absolute" integer NULL, "unverified_scene_numbering" bool NOT NULL DEFAULT (false), "last_searched_at" datetime NULL, "series_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "episode_states_items_episode_states" FOREIGN KEY ("series_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "episodestate_series_id_season_episode" to table: "episode_states"
CREATE UNIQUE INDEX "episodestate_series_id_season_episode" ON "episode_states" ("series_id", "season", "episode");
-- Create "extra_files" table
CREATE TABLE "extra_files" ("id" uuid NOT NULL, "kind" text NOT NULL, "path" text NOT NULL, "extension" text NOT NULL DEFAULT (''), "language" text NOT NULL DEFAULT (''), "language_tags" json NULL, "added" datetime NOT NULL, "last_updated" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "extra_files_items_extra_files" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "extrafile_item_id" to table: "extra_files"
CREATE INDEX "extrafile_item_id" ON "extra_files" ("item_id");
-- Create "folder_states" table
CREATE TABLE "folder_states" ("id" uuid NOT NULL, "path" text NOT NULL, "mod_time" datetime NOT NULL, "file_id" text NOT NULL DEFAULT (''), "entries" json NULL, "generation" integer NOT NULL DEFAULT (0), "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "folder_states_libraries_folder_states" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE);
-- Create index "folderstate_library_id_path" to table: "folder_states"
CREATE UNIQUE INDEX "folderstate_library_id_path" ON "folder_states" ("library_id", "path");
-- Create index "folderstate_library_id_generation" to table: "folder_states"
CREATE INDEX "folderstate_library_id_generation" ON "folder_states" ("library_id", "generation");
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
-- Create "images" table
CREATE TABLE "images" ("id" uuid NOT NULL, "kind" text NOT NULL, "index" integer NOT NULL DEFAULT (0), "path" text NOT NULL DEFAULT (''), "remote_url" text NOT NULL DEFAULT (''), "width" integer NOT NULL DEFAULT (0), "height" integer NOT NULL DEFAULT (0), "blurhash" text NOT NULL DEFAULT (''), "thumbhash" blob NULL, "item_id" uuid NULL, "person_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "images_items_images" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "images_people_images" FOREIGN KEY ("person_id") REFERENCES "people" ("id") ON DELETE CASCADE);
-- Create index "image_item_id_kind_index" to table: "images"
CREATE INDEX "image_item_id_kind_index" ON "images" ("item_id", "kind", "index");
-- Create index "image_person_id_kind_index" to table: "images"
CREATE INDEX "image_person_id_kind_index" ON "images" ("person_id", "kind", "index");
-- Create "invitations" table
CREATE TABLE "invitations" ("id" uuid NOT NULL, "token_hash" blob NOT NULL, "access" json NOT NULL, "max_uses" integer NOT NULL, "uses" integer NOT NULL DEFAULT (0), "expires_at" datetime NOT NULL, "created_at" datetime NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "invitations_users_invitations" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "invitations_token_hash_key" to table: "invitations"
CREATE UNIQUE INDEX "invitations_token_hash_key" ON "invitations" ("token_hash");
-- Create "issues" table
CREATE TABLE "issues" ("id" uuid NOT NULL, "type" text NOT NULL, "status" text NOT NULL, "season_number" integer NOT NULL DEFAULT (0), "episode_number" integer NOT NULL DEFAULT (0), "modified_by" uuid NULL, "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, "item_id" uuid NOT NULL, "created_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "issues_items_issues" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "issues_users_issues" FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "issue_created_by_created_at" to table: "issues"
CREATE INDEX "issue_created_by_created_at" ON "issues" ("created_by", "created_at");
-- Create index "issue_item_id" to table: "issues"
CREATE INDEX "issue_item_id" ON "issues" ("item_id");
-- Create index "issue_status" to table: "issues"
CREATE INDEX "issue_status" ON "issues" ("status");
-- Create index "issue_created_at" to table: "issues"
CREATE INDEX "issue_created_at" ON "issues" ("created_at");
-- Create "issue_comments" table
CREATE TABLE "issue_comments" ("id" uuid NOT NULL, "message" text NOT NULL, "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, "issue_id" uuid NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "issue_comments_issues_comments" FOREIGN KEY ("issue_id") REFERENCES "issues" ("id") ON DELETE CASCADE, CONSTRAINT "issue_comments_users_issue_comments" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "issuecomment_issue_id_created_at" to table: "issue_comments"
CREATE INDEX "issuecomment_issue_id_created_at" ON "issue_comments" ("issue_id", "created_at");
-- Create "items" table
CREATE TABLE "items" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL, "sort_name" text NOT NULL, "sort_key" text NOT NULL, "original_title" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "original_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "tagline" text NOT NULL DEFAULT (''), "path" text NOT NULL DEFAULT (''), "index_number" integer NULL, "parent_index_number" integer NULL, "index_number_end" integer NULL, "production_year" integer NOT NULL DEFAULT (0), "premiere_date" datetime NULL, "end_date" datetime NULL, "runtime" integer NOT NULL DEFAULT (0), "official_rating" text NOT NULL DEFAULT (''), "custom_rating" text NOT NULL DEFAULT (''), "parental_rating" integer NULL, "inherited_rating" integer NULL, "tag_keys" text NOT NULL DEFAULT (''), "inherited_tags" text NOT NULL DEFAULT (''), "community_rating" real NOT NULL DEFAULT (0), "critic_rating" real NOT NULL DEFAULT (0), "external_ids" json NULL, "production_locations" json NULL, "remote_trailers" json NULL, "collection_name" text NOT NULL DEFAULT (''), "aspect_ratio" text NOT NULL DEFAULT (''), "video_3d_format" text NOT NULL DEFAULT (''), "album" text NOT NULL DEFAULT (''), "series_status" text NOT NULL DEFAULT (''), "air_days" json NULL, "air_time" text NOT NULL DEFAULT (''), "display_order" text NOT NULL DEFAULT (''), "airs_before_season_number" integer NULL, "airs_after_season_number" integer NULL, "airs_before_episode_number" integer NULL, "metadata_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "locked" bool NOT NULL DEFAULT (false), "locked_fields" json NULL, "loudness" real NULL, "extra" text NOT NULL DEFAULT (''), "date_added" datetime NOT NULL, "file_modified" datetime NULL, "metadata_refreshed_at" datetime NULL, "scan_generation" integer NOT NULL DEFAULT (0), "missing_since" datetime NULL, "virtual" bool NOT NULL DEFAULT (false), "monitored" bool NOT NULL DEFAULT (false), "series_type" text NOT NULL DEFAULT (''), "absolute_number" integer NULL, "original_language" text NOT NULL DEFAULT (''), "digital_release_date" datetime NULL, "physical_release_date" datetime NULL, "alternative_titles" json NULL, "translated_titles" json NULL, "secondary_year" integer NOT NULL DEFAULT (0), "parent_id" uuid NULL, "owner_id" uuid NULL, "library_id" uuid NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "items_items_children" FOREIGN KEY ("parent_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_items_extras" FOREIGN KEY ("owner_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE, CONSTRAINT "items_users_playlists" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "item_library_id_path" to table: "items"
CREATE UNIQUE INDEX "item_library_id_path" ON "items" ("library_id", "path") WHERE path <> '';
-- Create index "item_parent_id_sort_key" to table: "items"
CREATE INDEX "item_parent_id_sort_key" ON "items" ("parent_id", "sort_key");
-- Create index "item_library_id_kind_sort_key" to table: "items"
CREATE INDEX "item_library_id_kind_sort_key" ON "items" ("library_id", "kind", "sort_key");
-- Create index "item_owner_id" to table: "items"
CREATE INDEX "item_owner_id" ON "items" ("owner_id");
-- Create index "item_date_added" to table: "items"
CREATE INDEX "item_date_added" ON "items" ("date_added");
-- Create index "item_library_id_scan_generation" to table: "items"
CREATE INDEX "item_library_id_scan_generation" ON "items" ("library_id", "scan_generation");
-- Create "item_links" table
CREATE TABLE "item_links" ("id" uuid NOT NULL, "ord" integer NOT NULL, "container_id" uuid NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "item_links_items_links" FOREIGN KEY ("container_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "item_links_items_linked_in" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "itemlink_container_id_ord" to table: "item_links"
CREATE INDEX "itemlink_container_id_ord" ON "item_links" ("container_id", "ord");
-- Create index "itemlink_item_id" to table: "item_links"
CREATE INDEX "itemlink_item_id" ON "item_links" ("item_id");
-- Create "item_values" table
CREATE TABLE "item_values" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "kind" text NOT NULL, "value" text NOT NULL, "value_key" text NOT NULL DEFAULT (''), "sort_key" text NOT NULL DEFAULT (''), "ord" integer NOT NULL, "item_id" uuid NOT NULL, CONSTRAINT "item_values_items_values" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "itemvalue_item_id_kind_ord" to table: "item_values"
CREATE UNIQUE INDEX "itemvalue_item_id_kind_ord" ON "item_values" ("item_id", "kind", "ord");
-- Create index "itemvalue_kind_value_key" to table: "item_values"
CREATE INDEX "itemvalue_kind_value_key" ON "item_values" ("kind", "value_key");
-- Create "jobs" table
CREATE TABLE "jobs" ("id" uuid NOT NULL, "kind" text NOT NULL, "payload" blob NULL, "unique_key" text NOT NULL DEFAULT (''), "state" text NOT NULL, "priority" integer NOT NULL DEFAULT (0), "attempts" integer NOT NULL DEFAULT (0), "max_attempts" integer NOT NULL, "run_at" datetime NOT NULL, "lease_owner" text NOT NULL DEFAULT (''), "lease_expires_at" datetime NULL, "last_error" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "finished_at" datetime NULL, PRIMARY KEY ("id"));
-- Create index "job_unique_key" to table: "jobs"
CREATE UNIQUE INDEX "job_unique_key" ON "jobs" ("unique_key") WHERE unique_key <> '' AND state = 'pending';
-- Create index "job_unique_key_running" to table: "jobs"
CREATE INDEX "job_unique_key_running" ON "jobs" ("unique_key") WHERE unique_key <> '' AND state = 'running';
-- Create index "job_state_kind_priority_run_at" to table: "jobs"
CREATE INDEX "job_state_kind_priority_run_at" ON "jobs" ("state", "kind", "priority", "run_at");
-- Create "libraries" table
CREATE TABLE "libraries" ("id" uuid NOT NULL, "name" text NOT NULL, "kind" text NOT NULL, "paths" json NOT NULL, "storage_classes" json NULL, "scan_interval" integer NOT NULL DEFAULT (0), "preferred_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "save_local_metadata" bool NOT NULL DEFAULT (false), "auto_collections" bool NOT NULL DEFAULT (false), "extract_trickplay" bool NOT NULL DEFAULT (false), "extract_chapter_images" bool NOT NULL DEFAULT (false), "analyze_loudness" bool NOT NULL DEFAULT (false), "download_lyrics" bool NOT NULL DEFAULT (false), "provider_order" json NULL, "virtual_episodes" json NULL, "hubs" json NULL, "scan_generation" integer NOT NULL DEFAULT (0), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "library_kind" to table: "libraries"
CREATE UNIQUE INDEX "library_kind" ON "libraries" ("kind") WHERE kind IN ('collections', 'playlists');
-- Create "media_segments" table
CREATE TABLE "media_segments" ("id" uuid NOT NULL, "kind" text NOT NULL, "start" integer NOT NULL, "end" integer NOT NULL, "provider" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_segments_items_segments" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "mediasegment_item_id_start" to table: "media_segments"
CREATE INDEX "mediasegment_item_id_start" ON "media_segments" ("item_id", "start");
-- Create "media_sources" table
CREATE TABLE "media_sources" ("id" uuid NOT NULL, "ord" integer NOT NULL, "path" text NOT NULL, "parts" json NULL, "disc" text NOT NULL DEFAULT (''), "name" text NOT NULL DEFAULT (''), "container" text NOT NULL DEFAULT (''), "size" integer NOT NULL DEFAULT (0), "modified" datetime NULL, "duration" integer NOT NULL DEFAULT (0), "bitrate" integer NOT NULL DEFAULT (0), "streams" json NULL, "chapters" json NULL, "keyframes" json NULL, "probed_at" datetime NULL, "file_hash" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "mediasource_item_id_ord" to table: "media_sources"
CREATE UNIQUE INDEX "mediasource_item_id_ord" ON "media_sources" ("item_id", "ord");
-- Create "pending_releases" table
CREATE TABLE "pending_releases" ("id" uuid NOT NULL, "item_ids" json NULL, "title" text NOT NULL DEFAULT (''), "added" datetime NOT NULL, "reason" text NOT NULL, "release" text NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "pending_releases_items_pending_releases" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "pendingrelease_item_id_added" to table: "pending_releases"
CREATE INDEX "pendingrelease_item_id_added" ON "pending_releases" ("item_id", "added");
-- Create "people" table
CREATE TABLE "people" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "sort_name" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "sort_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "birth_date" datetime NULL, "death_date" datetime NULL, "birth_place" text NOT NULL DEFAULT (''), "external_ids" json NULL, PRIMARY KEY ("id"));
-- Create index "person_name_key" to table: "people"
CREATE INDEX "person_name_key" ON "people" ("name_key");
-- Create "plugin_configs" table
CREATE TABLE "plugin_configs" ("id" uuid NOT NULL, "plugin_id" text NOT NULL, "config" text NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "plugin_configs_plugin_id_key" to table: "plugin_configs"
CREATE UNIQUE INDEX "plugin_configs_plugin_id_key" ON "plugin_configs" ("plugin_id");
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
-- Create "requests" table
CREATE TABLE "requests" ("id" uuid NOT NULL, "kind" text NOT NULL, "tmdb_id" integer NOT NULL, "tvdb_id" integer NOT NULL DEFAULT (0), "title" text NOT NULL DEFAULT (''), "year" integer NOT NULL DEFAULT (0), "item_id" uuid NULL, "seasons" json NULL, "season_count" integer NOT NULL DEFAULT (0), "status" text NOT NULL, "modified_by" uuid NULL, "ignore_quota" bool NOT NULL DEFAULT (false), "message" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, "requested_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "requests_users_requests" FOREIGN KEY ("requested_by") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "request_requested_by_created_at" to table: "requests"
CREATE INDEX "request_requested_by_created_at" ON "requests" ("requested_by", "created_at");
-- Create index "request_kind_tmdb_id" to table: "requests"
CREATE INDEX "request_kind_tmdb_id" ON "requests" ("kind", "tmdb_id");
-- Create index "request_item_id" to table: "requests"
CREATE INDEX "request_item_id" ON "requests" ("item_id");
-- Create index "request_status" to table: "requests"
CREATE INDEX "request_status" ON "requests" ("status");
-- Create index "request_created_at" to table: "requests"
CREATE INDEX "request_created_at" ON "requests" ("created_at");
-- Create "settings" table
CREATE TABLE "settings" ("id" uuid NOT NULL, "key" text NOT NULL, "value" text NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "settings_key_key" to table: "settings"
CREATE UNIQUE INDEX "settings_key_key" ON "settings" ("key");
-- Create "storage_jobs" table
CREATE TABLE "storage_jobs" ("id" uuid NOT NULL, "kind" text NOT NULL, "key" text NOT NULL, "remote" text NOT NULL, "execute_id" text NULL, "job_id" integer NULL, "data" json NULL, "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "storagejob_kind_key" to table: "storage_jobs"
CREATE UNIQUE INDEX "storagejob_kind_key" ON "storage_jobs" ("kind", "key");
-- Create "subtitle_blocklist" table
CREATE TABLE "subtitle_blocklist" ("id" uuid NOT NULL, "parent_id" uuid NULL, "provider" text NOT NULL, "subtitle_id" text NOT NULL, "language" text NOT NULL, "forced" bool NOT NULL DEFAULT (false), "hearing_impaired" bool NOT NULL DEFAULT (false), "date" datetime NOT NULL, "message" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_blocklist_items_subtitle_blocklist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "subtitleblocklistentry_item_id_date" to table: "subtitle_blocklist"
CREATE INDEX "subtitleblocklistentry_item_id_date" ON "subtitle_blocklist" ("item_id", "date");
-- Create index "subtitleblocklistentry_date" to table: "subtitle_blocklist"
CREATE INDEX "subtitleblocklistentry_date" ON "subtitle_blocklist" ("date");
-- Create "subtitle_history" table
CREATE TABLE "subtitle_history" ("id" uuid NOT NULL, "action" text NOT NULL, "parent_id" uuid NULL, "date" datetime NOT NULL, "language" text NOT NULL, "forced" bool NOT NULL DEFAULT (false), "hearing_impaired" bool NOT NULL DEFAULT (false), "provider" text NOT NULL DEFAULT (''), "subtitle_id" text NOT NULL DEFAULT (''), "score" integer NOT NULL DEFAULT (0), "max_score" integer NOT NULL DEFAULT (0), "path" text NOT NULL DEFAULT (''), "video_path" text NOT NULL DEFAULT (''), "description" text NOT NULL DEFAULT (''), "matched" json NULL, "not_matched" json NULL, "upgraded_from" uuid NULL, "translation" json NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_history_items_subtitle_history" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "subtitlehistoryevent_item_id_date" to table: "subtitle_history"
CREATE INDEX "subtitlehistoryevent_item_id_date" ON "subtitle_history" ("item_id", "date");
-- Create index "subtitlehistoryevent_parent_id_date" to table: "subtitle_history"
CREATE INDEX "subtitlehistoryevent_parent_id_date" ON "subtitle_history" ("parent_id", "date");
-- Create index "subtitlehistoryevent_date" to table: "subtitle_history"
CREATE INDEX "subtitlehistoryevent_date" ON "subtitle_history" ("date");
-- Create "subtitle_profiles" table
CREATE TABLE "subtitle_profiles" ("id" uuid NOT NULL, "name" text NOT NULL, "items" json NOT NULL, "cutoff" integer NOT NULL DEFAULT (0), "must_contain" json NULL, "must_not_contain" json NULL, "original_format" bool NOT NULL DEFAULT (false), "tag" text NOT NULL DEFAULT (''), PRIMARY KEY ("id"));
-- Create index "subtitle_profiles_name_key" to table: "subtitle_profiles"
CREATE UNIQUE INDEX "subtitle_profiles_name_key" ON "subtitle_profiles" ("name");
-- Create "subtitle_profile_assignments" table
CREATE TABLE "subtitle_profile_assignments" ("id" uuid NOT NULL, "item_id" uuid NULL, "library_id" uuid NULL, "profile_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_profile_assignments_items_subtitle_profile" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "subtitle_profile_assignments_libraries_subtitle_profile" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE, CONSTRAINT "subtitle_profile_assignments_subtitle_profiles_assignments" FOREIGN KEY ("profile_id") REFERENCES "subtitle_profiles" ("id") ON DELETE CASCADE);
-- Create index "subtitle_profile_assignments_item_id_key" to table: "subtitle_profile_assignments"
CREATE UNIQUE INDEX "subtitle_profile_assignments_item_id_key" ON "subtitle_profile_assignments" ("item_id");
-- Create index "subtitle_profile_assignments_library_id_key" to table: "subtitle_profile_assignments"
CREATE UNIQUE INDEX "subtitle_profile_assignments_library_id_key" ON "subtitle_profile_assignments" ("library_id");
-- Create "subtitle_search_attempts" table
CREATE TABLE "subtitle_search_attempts" ("id" uuid NOT NULL, "language" text NOT NULL, "forced" bool NOT NULL DEFAULT (false), "hearing_impaired" bool NOT NULL DEFAULT (false), "first" datetime NOT NULL, "last" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_search_attempts_items_subtitle_search_attempts" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "subtitlesearchattempt_item_id_language_forced_hearing_impaired" to table: "subtitle_search_attempts"
CREATE UNIQUE INDEX "subtitlesearchattempt_item_id_language_forced_hearing_impaired" ON "subtitle_search_attempts" ("item_id", "language", "forced", "hearing_impaired");
-- Create "subtitle_wanted" table
CREATE TABLE "subtitle_wanted" ("id" uuid NOT NULL, "kind" text NOT NULL, "library_id" uuid NOT NULL, "parent_id" uuid NULL, "path" text NOT NULL, "missing" json NOT NULL, "updated" datetime NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_wanted_items_subtitle_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "subtitle_wanted_item_id_key" to table: "subtitle_wanted"
CREATE UNIQUE INDEX "subtitle_wanted_item_id_key" ON "subtitle_wanted" ("item_id");
-- Create index "subtitlewanted_kind_library_id_path" to table: "subtitle_wanted"
CREATE INDEX "subtitlewanted_kind_library_id_path" ON "subtitle_wanted" ("kind", "library_id", "path");
-- Create "trickplay" table
CREATE TABLE "trickplay" ("id" uuid NOT NULL, "width" integer NOT NULL, "height" integer NOT NULL, "tile_width" integer NOT NULL, "tile_height" integer NOT NULL, "thumbnail_count" integer NOT NULL, "interval" integer NOT NULL, "bandwidth" integer NOT NULL DEFAULT (0), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "trickplay_items_trickplay" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "trickplay_item_id_width" to table: "trickplay"
CREATE UNIQUE INDEX "trickplay_item_id_width" ON "trickplay" ("item_id", "width");
-- Create "users" table
CREATE TABLE "users" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "kind" text NOT NULL, "auth_provider" text NOT NULL DEFAULT (''), "account_id" text NULL, "restriction" text NOT NULL DEFAULT (''), "disabled" bool NOT NULL DEFAULT (false), "access" json NULL, "policy" json NOT NULL, "preferences" json NOT NULL, "created_at" datetime NOT NULL, "last_login_at" datetime NULL, PRIMARY KEY ("id"));
-- Create index "users_name_key_key" to table: "users"
CREATE UNIQUE INDEX "users_name_key_key" ON "users" ("name_key");
-- Create index "users_account_id_key" to table: "users"
CREATE UNIQUE INDEX "users_account_id_key" ON "users" ("account_id");
-- Create index "user_kind" to table: "users"
CREATE UNIQUE INDEX "user_kind" ON "users" ("kind") WHERE kind = 'administrator';
-- Create "user_data" table
CREATE TABLE "user_data" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "played" bool NOT NULL DEFAULT (false), "play_count" integer NOT NULL DEFAULT (0), "position" integer NOT NULL DEFAULT (0), "audio_stream" integer NULL, "subtitle_stream" integer NULL, "favorite" bool NOT NULL DEFAULT (false), "rating" real NULL, "last_played_at" datetime NULL, "updated_at" datetime NOT NULL, "item_id" uuid NOT NULL, "user_id" uuid NOT NULL, CONSTRAINT "user_data_items_user_data" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "user_data_users_user_data" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "userdata_user_id_item_id" to table: "user_data"
CREATE UNIQUE INDEX "userdata_user_id_item_id" ON "user_data" ("user_id", "item_id");
-- Create "wanted" table
CREATE TABLE "wanted" ("id" uuid NOT NULL, "minimum_availability" text NOT NULL DEFAULT (''), "monitor_new_items" text NOT NULL DEFAULT (''), "season_folder" bool NOT NULL DEFAULT (false), "tags" json NULL, "added_at" datetime NOT NULL, "last_searched_at" datetime NULL, "item_id" uuid NOT NULL, "quality_profile_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "wanted_items_wanted" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "wanted_quality_profiles_wanted" FOREIGN KEY ("quality_profile_id") REFERENCES "quality_profiles" ("id") ON DELETE NO ACTION);
-- Create index "wanted_item_id_key" to table: "wanted"
CREATE UNIQUE INDEX "wanted_item_id_key" ON "wanted" ("item_id");
