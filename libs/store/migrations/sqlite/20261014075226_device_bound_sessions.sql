-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_auth_sessions" table
CREATE TABLE "new_auth_sessions" ("id" uuid NOT NULL, "device_key" blob NOT NULL, "device_id" text NOT NULL, "device_name" text NOT NULL DEFAULT (''), "client" text NOT NULL DEFAULT (''), "client_version" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "last_seen_at" datetime NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_sessions_users_auth_sessions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Sessions without a device key are dropped: their devices sign in again.
-- Drop "auth_sessions" table after copying rows
DROP TABLE "auth_sessions";
-- Rename temporary table "new_auth_sessions" to "auth_sessions"
ALTER TABLE "new_auth_sessions" RENAME TO "auth_sessions";
-- Create index "authsession_user_id_device_id" to table: "auth_sessions"
CREATE UNIQUE INDEX "authsession_user_id_device_id" ON "auth_sessions" ("user_id", "device_id");
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
