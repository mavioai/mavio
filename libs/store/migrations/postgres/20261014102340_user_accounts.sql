-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "account_id" character varying NULL;
-- Create index "users_account_id_key" to table: "users"
CREATE UNIQUE INDEX "users_account_id_key" ON "users" ("account_id");
