-- Create "blocked_titles" table
CREATE TABLE "blocked_titles" ("id" uuid NOT NULL, "kind" text NOT NULL, "tmdb_id" integer NOT NULL, "title" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "blocked_titles_users_blocked_titles" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE SET NULL);
-- Create index "blockedtitle_kind_tmdb_id" to table: "blocked_titles"
CREATE UNIQUE INDEX "blockedtitle_kind_tmdb_id" ON "blocked_titles" ("kind", "tmdb_id");
-- Create index "blockedtitle_created_at" to table: "blocked_titles"
CREATE INDEX "blockedtitle_created_at" ON "blocked_titles" ("created_at");
