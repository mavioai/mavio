-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "original_language" character varying NOT NULL DEFAULT '', ADD COLUMN "digital_release_date" timestamptz NULL, ADD COLUMN "physical_release_date" timestamptz NULL, ADD COLUMN "alternative_titles" jsonb NULL, ADD COLUMN "translated_titles" jsonb NULL, ADD COLUMN "secondary_year" bigint NOT NULL DEFAULT 0;
