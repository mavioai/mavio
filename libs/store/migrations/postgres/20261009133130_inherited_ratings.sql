-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "inherited_rating" bigint NOT NULL DEFAULT 0;
