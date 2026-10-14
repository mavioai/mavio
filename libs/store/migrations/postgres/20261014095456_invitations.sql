-- Modify "auth_sessions" table
ALTER TABLE "auth_sessions" ADD COLUMN "switcher_id" uuid NULL;
-- Create "invitations" table
CREATE TABLE "invitations" ("id" uuid NOT NULL, "token_hash" bytea NOT NULL, "access" jsonb NOT NULL, "max_uses" bigint NOT NULL, "uses" bigint NOT NULL DEFAULT 0, "expires_at" timestamptz NOT NULL, "created_at" timestamptz NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "invitations_users_invitations" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "invitations_token_hash_key" to table: "invitations"
CREATE UNIQUE INDEX "invitations_token_hash_key" ON "invitations" ("token_hash");
