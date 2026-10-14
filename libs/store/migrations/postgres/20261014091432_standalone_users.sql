-- Add "kind" and "restriction" to the "users" table
ALTER TABLE "users" ADD COLUMN "kind" character varying NOT NULL DEFAULT 'member', ADD COLUMN "restriction" character varying NOT NULL DEFAULT '';
UPDATE "users" SET "kind" = 'administrator' WHERE "admin";
ALTER TABLE "users" ALTER COLUMN "kind" DROP DEFAULT;
-- Create index "user_kind" to table: "users"
CREATE UNIQUE INDEX "user_kind" ON "users" ("kind") WHERE ((kind)::text = 'administrator'::text);
-- Create "credentials" table
CREATE TABLE "credentials" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "name" character varying NOT NULL DEFAULT '', "data" character varying NOT NULL, "created_at" timestamptz NOT NULL, "last_used_at" timestamptz NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "credentials_users_credentials" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "credential_user_id_kind" to table: "credentials"
CREATE UNIQUE INDEX "credential_user_id_kind" ON "credentials" ("user_id", "kind") WHERE ((kind)::text <> 'passkey'::text);
-- Move the users' password hashes into "credentials"
INSERT INTO "credentials" ("id", "kind", "data", "created_at", "user_id") SELECT gen_random_uuid(), 'password', "password_hash", "created_at", "id" FROM "users" WHERE "password_hash" <> '';
-- Drop the old columns of "users"
ALTER TABLE "users" DROP COLUMN "password_hash", DROP COLUMN "admin";
