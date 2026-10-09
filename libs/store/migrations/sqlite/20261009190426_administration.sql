-- Create "api_keys" table
CREATE TABLE "api_keys" ("id" uuid NOT NULL, "name" text NOT NULL, "token_hash" blob NOT NULL, "created_at" datetime NOT NULL, "last_used_at" datetime NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "api_keys_users_api_keys" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "api_keys_token_hash_key" to table: "api_keys"
CREATE UNIQUE INDEX "api_keys_token_hash_key" ON "api_keys" ("token_hash");
-- Create "activities" table
CREATE TABLE "activities" ("id" uuid NOT NULL, "time" datetime NOT NULL, "type" text NOT NULL, "severity" text NOT NULL, "title" text NOT NULL, "message" text NOT NULL DEFAULT (''), "user_id" uuid NULL, "item_id" uuid NULL, "attributes" json NULL, PRIMARY KEY ("id"));
-- Create index "activity_time" to table: "activities"
CREATE INDEX "activity_time" ON "activities" ("time");
-- Create "settings" table
CREATE TABLE "settings" ("id" uuid NOT NULL, "key" text NOT NULL, "value" text NOT NULL, "updated_at" datetime NOT NULL, PRIMARY KEY ("id"));
-- Create index "settings_key_key" to table: "settings"
CREATE UNIQUE INDEX "settings_key_key" ON "settings" ("key");
