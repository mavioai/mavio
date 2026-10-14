-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "tag_keys" text NOT NULL DEFAULT '', ADD COLUMN "inherited_tags" text NOT NULL DEFAULT '';
-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "access" jsonb NULL;
-- Move the users' library access out of their policies
UPDATE "users" SET "access" = "policy";
