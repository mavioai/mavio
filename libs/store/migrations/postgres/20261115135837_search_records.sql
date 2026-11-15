-- Create "search_records" table
CREATE TABLE "search_records" ("id" uuid NOT NULL, "query" character varying NOT NULL, "text" character varying NOT NULL, "voter" character varying NOT NULL, "searched_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "searchrecord_query_voter" to table: "search_records"
CREATE UNIQUE INDEX "searchrecord_query_voter" ON "search_records" ("query", "voter");
-- Create index "searchrecord_searched_at" to table: "search_records"
CREATE INDEX "searchrecord_searched_at" ON "search_records" ("searched_at");
