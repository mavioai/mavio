-- Modify "items" table
ALTER TABLE "items" ALTER COLUMN "parental_rating" DROP NOT NULL, ALTER COLUMN "parental_rating" DROP DEFAULT, ALTER COLUMN "inherited_rating" DROP NOT NULL, ALTER COLUMN "inherited_rating" DROP DEFAULT;
