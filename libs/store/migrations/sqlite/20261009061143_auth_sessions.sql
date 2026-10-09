-- Create "auth_sessions" table
CREATE TABLE "auth_sessions" ("id" uuid NOT NULL, "token_hash" blob NOT NULL, "device_id" text NOT NULL, "device_name" text NOT NULL DEFAULT (''), "client" text NOT NULL DEFAULT (''), "client_version" text NOT NULL DEFAULT (''), "created_at" datetime NOT NULL, "last_seen_at" datetime NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_sessions_users_auth_sessions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "auth_sessions_token_hash_key" to table: "auth_sessions"
CREATE UNIQUE INDEX "auth_sessions_token_hash_key" ON "auth_sessions" ("token_hash");
-- Create index "authsession_user_id_device_id" to table: "auth_sessions"
CREATE UNIQUE INDEX "authsession_user_id_device_id" ON "auth_sessions" ("user_id", "device_id");
