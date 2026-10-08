-- Create "credits" table
CREATE TABLE "credits" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "kind" text NOT NULL, "role" text NOT NULL DEFAULT (''), "ord" integer NOT NULL DEFAULT (0), "item_id" uuid NOT NULL, "person_id" uuid NOT NULL, CONSTRAINT "credits_items_credits" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "credits_people_credits" FOREIGN KEY ("person_id") REFERENCES "people" ("id") ON DELETE CASCADE);
-- Create index "credit_item_id_ord" to table: "credits"
CREATE INDEX "credit_item_id_ord" ON "credits" ("item_id", "ord");
-- Create index "credit_person_id" to table: "credits"
CREATE INDEX "credit_person_id" ON "credits" ("person_id");
-- Create "images" table
CREATE TABLE "images" ("id" uuid NOT NULL, "kind" text NOT NULL, "index" integer NOT NULL DEFAULT (0), "path" text NOT NULL DEFAULT (''), "remote_url" text NOT NULL DEFAULT (''), "width" integer NOT NULL DEFAULT (0), "height" integer NOT NULL DEFAULT (0), "blurhash" text NOT NULL DEFAULT (''), "thumbhash" blob NULL, "item_id" uuid NULL, "person_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "images_items_images" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "images_people_images" FOREIGN KEY ("person_id") REFERENCES "people" ("id") ON DELETE CASCADE);
-- Create index "image_item_id_kind_index" to table: "images"
CREATE INDEX "image_item_id_kind_index" ON "images" ("item_id", "kind", "index");
-- Create index "image_person_id_kind_index" to table: "images"
CREATE INDEX "image_person_id_kind_index" ON "images" ("person_id", "kind", "index");
-- Create "items" table
CREATE TABLE "items" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL, "sort_name" text NOT NULL, "sort_key" text NOT NULL, "original_title" text NOT NULL DEFAULT (''), "search_key" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "tagline" text NOT NULL DEFAULT (''), "path" text NOT NULL DEFAULT (''), "index_number" integer NULL, "parent_index_number" integer NULL, "index_number_end" integer NULL, "production_year" integer NOT NULL DEFAULT (0), "premiere_date" datetime NULL, "end_date" datetime NULL, "runtime" integer NOT NULL DEFAULT (0), "official_rating" text NOT NULL DEFAULT (''), "parental_rating" integer NOT NULL DEFAULT (0), "community_rating" real NOT NULL DEFAULT (0), "critic_rating" real NOT NULL DEFAULT (0), "external_ids" json NULL, "series_status" text NOT NULL DEFAULT (''), "extra" text NOT NULL DEFAULT (''), "date_added" datetime NOT NULL, "file_modified" datetime NULL, "metadata_refreshed_at" datetime NULL, "parent_id" uuid NULL, "owner_id" uuid NULL, "library_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "items_items_children" FOREIGN KEY ("parent_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_items_extras" FOREIGN KEY ("owner_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON DELETE CASCADE);
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
-- Create "item_values" table
CREATE TABLE "item_values" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "kind" text NOT NULL, "value" text NOT NULL, "ord" integer NOT NULL, "item_id" uuid NOT NULL, CONSTRAINT "item_values_items_values" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "itemvalue_item_id_kind_ord" to table: "item_values"
CREATE UNIQUE INDEX "itemvalue_item_id_kind_ord" ON "item_values" ("item_id", "kind", "ord");
-- Create index "itemvalue_kind_value" to table: "item_values"
CREATE INDEX "itemvalue_kind_value" ON "item_values" ("kind", "value");
-- Create "jobs" table
CREATE TABLE "jobs" ("id" uuid NOT NULL, "kind" text NOT NULL, "payload" blob NULL, "unique_key" text NOT NULL DEFAULT (''), "state" text NOT NULL, "priority" integer NOT NULL DEFAULT (0), "attempts" integer NOT NULL DEFAULT (0), "max_attempts" integer NOT NULL, "run_at" datetime NOT NULL, "lease_owner" text NOT NULL DEFAULT (''), "lease_expires_at" datetime NULL, "last_error" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "finished_at" datetime NULL, PRIMARY KEY ("id"));
-- Create index "job_unique_key" to table: "jobs"
CREATE UNIQUE INDEX "job_unique_key" ON "jobs" ("unique_key") WHERE unique_key <> '' AND state IN ('pending', 'running');
-- Create index "job_state_kind_priority_run_at" to table: "jobs"
CREATE INDEX "job_state_kind_priority_run_at" ON "jobs" ("state", "kind", "priority", "run_at");
-- Create "libraries" table
CREATE TABLE "libraries" ("id" uuid NOT NULL, "name" text NOT NULL, "kind" text NOT NULL, "paths" json NOT NULL, "scan_interval" integer NOT NULL DEFAULT (0), "preferred_language" text NOT NULL DEFAULT (''), "metadata_country" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create "media_sources" table
CREATE TABLE "media_sources" ("id" uuid NOT NULL, "ord" integer NOT NULL, "path" text NOT NULL, "name" text NOT NULL DEFAULT (''), "container" text NOT NULL DEFAULT (''), "size" integer NOT NULL DEFAULT (0), "duration" integer NOT NULL DEFAULT (0), "bitrate" integer NOT NULL DEFAULT (0), "streams" json NULL, "chapters" json NULL, "keyframes" json NULL, "probed_at" datetime NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "mediasource_item_id_ord" to table: "media_sources"
CREATE UNIQUE INDEX "mediasource_item_id_ord" ON "media_sources" ("item_id", "ord");
-- Create "people" table
CREATE TABLE "people" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "sort_name" text NOT NULL DEFAULT (''), "overview" text NOT NULL DEFAULT (''), "birth_date" datetime NULL, "death_date" datetime NULL, "birth_place" text NOT NULL DEFAULT (''), "external_ids" json NULL, PRIMARY KEY ("id"));
-- Create index "person_name_key" to table: "people"
CREATE INDEX "person_name_key" ON "people" ("name_key");
-- Create "users" table
CREATE TABLE "users" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "password_hash" text NOT NULL DEFAULT (''), "auth_provider" text NOT NULL DEFAULT (''), "admin" bool NOT NULL DEFAULT (false), "disabled" bool NOT NULL DEFAULT (false), "policy" json NOT NULL, "preferences" json NOT NULL, "created_at" datetime NOT NULL, "last_login_at" datetime NULL, PRIMARY KEY ("id"));
-- Create index "users_name_key_key" to table: "users"
CREATE UNIQUE INDEX "users_name_key_key" ON "users" ("name_key");
-- Create "user_data" table
CREATE TABLE "user_data" ("id" integer NOT NULL PRIMARY KEY AUTOINCREMENT, "played" bool NOT NULL DEFAULT (false), "play_count" integer NOT NULL DEFAULT (0), "position" integer NOT NULL DEFAULT (0), "audio_stream" integer NULL, "subtitle_stream" integer NULL, "favorite" bool NOT NULL DEFAULT (false), "rating" real NULL, "last_played_at" datetime NULL, "updated_at" datetime NOT NULL, "item_id" uuid NOT NULL, "user_id" uuid NOT NULL, CONSTRAINT "user_data_items_user_data" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE, CONSTRAINT "user_data_users_user_data" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "userdata_user_id_item_id" to table: "user_data"
CREATE UNIQUE INDEX "userdata_user_id_item_id" ON "user_data" ("user_id", "item_id");
