-- Create "proxies" table
CREATE TABLE "proxies" ("id" uuid NOT NULL, "name" text NOT NULL, "name_key" text NOT NULL, "kind" text NOT NULL, "host" text NOT NULL, "port" integer NOT NULL, "username" text NOT NULL DEFAULT (''), "password" text NOT NULL DEFAULT (''), "address_key" text NOT NULL, PRIMARY KEY ("id"));
-- Create index "proxies_name_key_key" to table: "proxies"
CREATE UNIQUE INDEX "proxies_name_key_key" ON "proxies" ("name_key");
-- Create index "proxies_address_key_key" to table: "proxies"
CREATE UNIQUE INDEX "proxies_address_key_key" ON "proxies" ("address_key");
