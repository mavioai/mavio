-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_users" table
CREATE TABLE "new_users" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "kind" text NOT NULL, "auth_provider" text NOT NULL DEFAULT (''), "account_id" text NULL, "restriction" text NOT NULL DEFAULT (''), "disabled" bool NOT NULL DEFAULT (false), "access" json NULL, "policy" json NOT NULL, "preferences" json NOT NULL, "created_at" datetime NOT NULL, "last_login_at" datetime NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "users" to new temporary table "new_users"
INSERT INTO "new_users" ("id", "name", "name_key", "kind", "auth_provider", "restriction", "disabled", "access", "policy", "preferences", "created_at", "last_login_at") SELECT "id", "name", "name_key", "kind", "auth_provider", "restriction", "disabled", "access", "policy", "preferences", "created_at", "last_login_at" FROM "users";
-- Drop "users" table after copying rows
DROP TABLE "users";
-- Rename temporary table "new_users" to "users"
ALTER TABLE "new_users" RENAME TO "users";
-- Create index "users_name_key_key" to table: "users"
CREATE UNIQUE INDEX "users_name_key_key" ON "users" ("name_key");
-- Create index "users_account_id_key" to table: "users"
CREATE UNIQUE INDEX "users_account_id_key" ON "users" ("account_id");
-- Create index "user_kind" to table: "users"
CREATE UNIQUE INDEX "user_kind" ON "users" ("kind") WHERE kind = 'administrator';
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
