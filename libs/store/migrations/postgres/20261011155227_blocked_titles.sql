-- Create "blocked_titles" table
CREATE TABLE "blocked_titles" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "tmdb_id" bigint NOT NULL, "title" character varying NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "blocked_titles_users_blocked_titles" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL);
-- Create index "blockedtitle_created_at" to table: "blocked_titles"
CREATE INDEX "blockedtitle_created_at" ON "blocked_titles" ("created_at");
-- Create index "blockedtitle_kind_tmdb_id" to table: "blocked_titles"
CREATE UNIQUE INDEX "blockedtitle_kind_tmdb_id" ON "blocked_titles" ("kind", "tmdb_id");
