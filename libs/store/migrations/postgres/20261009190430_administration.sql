-- Create "activities" table
CREATE TABLE "activities" ("id" uuid NOT NULL, "time" timestamptz NOT NULL, "type" character varying NOT NULL, "severity" character varying NOT NULL, "title" character varying NOT NULL, "message" text NOT NULL DEFAULT '', "user_id" uuid NULL, "item_id" uuid NULL, "attributes" jsonb NULL, PRIMARY KEY ("id"));
-- Create index "activity_time" to table: "activities"
CREATE INDEX "activity_time" ON "activities" ("time");
-- Create "settings" table
CREATE TABLE "settings" ("id" uuid NOT NULL, "key" character varying NOT NULL, "value" text NOT NULL, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "settings_key_key" to table: "settings"
CREATE UNIQUE INDEX "settings_key_key" ON "settings" ("key");
-- Create "api_keys" table
CREATE TABLE "api_keys" ("id" uuid NOT NULL, "name" character varying NOT NULL, "token_hash" bytea NOT NULL, "created_at" timestamptz NOT NULL, "last_used_at" timestamptz NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "api_keys_users_api_keys" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "api_keys_token_hash_key" to table: "api_keys"
CREATE UNIQUE INDEX "api_keys_token_hash_key" ON "api_keys" ("token_hash");
