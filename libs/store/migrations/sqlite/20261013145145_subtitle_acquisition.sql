-- Create "subtitle_blocklist" table
CREATE TABLE "subtitle_blocklist" ("id" uuid NOT NULL, "parent_id" uuid NULL, "provider" text NOT NULL, "subtitle_id" text NOT NULL, "language" text NOT NULL, "forced" bool NOT NULL DEFAULT (false), "hearing_impaired" bool NOT NULL DEFAULT (false), "date" datetime NOT NULL, "message" text NOT NULL DEFAULT (''), "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_blocklist_items_subtitle_blocklist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "subtitleblocklistentry_item_id_date" to table: "subtitle_blocklist"
CREATE INDEX "subtitleblocklistentry_item_id_date" ON "subtitle_blocklist" ("item_id", "date");
-- Create index "subtitleblocklistentry_date" to table: "subtitle_blocklist"
CREATE INDEX "subtitleblocklistentry_date" ON "subtitle_blocklist" ("date");
-- Create "subtitle_history" table
CREATE TABLE "subtitle_history" ("id" uuid NOT NULL, "action" text NOT NULL, "parent_id" uuid NULL, "date" datetime NOT NULL, "language" text NOT NULL, "forced" bool NOT NULL DEFAULT (false), "hearing_impaired" bool NOT NULL DEFAULT (false), "provider" text NOT NULL DEFAULT (''), "subtitle_id" text NOT NULL DEFAULT (''), "score" integer NOT NULL DEFAULT (0), "max_score" integer NOT NULL DEFAULT (0), "path" text NOT NULL DEFAULT (''), "video_path" text NOT NULL DEFAULT (''), "description" text NOT NULL DEFAULT (''), "matched" json NULL, "not_matched" json NULL, "upgraded_from" uuid NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "subtitle_history_items_subtitle_history" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
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
