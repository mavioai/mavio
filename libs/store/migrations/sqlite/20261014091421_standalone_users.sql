-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_users" table
CREATE TABLE "new_users" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "kind" text NOT NULL, "auth_provider" text NOT NULL DEFAULT (''), "restriction" text NOT NULL DEFAULT (''), "disabled" bool NOT NULL DEFAULT (false), "policy" json NOT NULL, "preferences" json NOT NULL, "created_at" datetime NOT NULL, "last_login_at" datetime NULL, PRIMARY KEY ("id"));
-- Copy rows from old table "users" to new temporary table "new_users"
INSERT INTO "new_users" ("id", "name", "name_key", "kind", "auth_provider", "disabled", "policy", "preferences", "created_at", "last_login_at") SELECT "id", "name", "name_key", CASE WHEN "admin" THEN 'administrator' ELSE 'member' END, "auth_provider", "disabled", "policy", "preferences", "created_at", "last_login_at" FROM "users";
-- Create "credentials" table
CREATE TABLE "credentials" ("id" uuid NOT NULL, "kind" text NOT NULL, "name" text NOT NULL DEFAULT (''), "data" text NOT NULL, "created_at" datetime NOT NULL, "last_used_at" datetime NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "credentials_users_credentials" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Move the users' password hashes into "credentials"
INSERT INTO "credentials" ("id", "kind", "data", "created_at", "user_id") SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)), 2) || '-' || substr('89ab', 1 + abs(random()) % 4, 1) || substr(hex(randomblob(2)), 2) || '-' || hex(randomblob(6))), 'password', "password_hash", "created_at", "id" FROM "users" WHERE "password_hash" <> '';
-- Drop "users" table after copying rows
DROP TABLE "users";
-- Rename temporary table "new_users" to "users"
ALTER TABLE "new_users" RENAME TO "users";
-- Create index "users_name_key_key" to table: "users"
CREATE UNIQUE INDEX "users_name_key_key" ON "users" ("name_key");
-- Create index "user_kind" to table: "users"
CREATE UNIQUE INDEX "user_kind" ON "users" ("kind") WHERE kind = 'administrator';
-- Create index "credential_user_id_kind" to table: "credentials"
CREATE UNIQUE INDEX "credential_user_id_kind" ON "credentials" ("user_id", "kind") WHERE kind <> 'passkey';
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
