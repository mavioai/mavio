-- Add column "tags" to table: "blocked_titles"
ALTER TABLE "blocked_titles" ADD COLUMN "tags" json NULL;
