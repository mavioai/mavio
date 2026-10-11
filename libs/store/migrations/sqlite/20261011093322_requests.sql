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
