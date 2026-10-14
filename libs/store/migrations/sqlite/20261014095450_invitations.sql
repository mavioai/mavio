-- Add column "switcher_id" to table: "auth_sessions"
ALTER TABLE "auth_sessions" ADD COLUMN "switcher_id" uuid NULL;
-- Create "invitations" table
CREATE TABLE "invitations" ("id" uuid NOT NULL, "token_hash" blob NOT NULL, "access" json NOT NULL, "max_uses" integer NOT NULL, "uses" integer NOT NULL DEFAULT (0), "expires_at" datetime NOT NULL, "created_at" datetime NOT NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "invitations_users_invitations" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "invitations_token_hash_key" to table: "invitations"
CREATE UNIQUE INDEX "invitations_token_hash_key" ON "invitations" ("token_hash");
