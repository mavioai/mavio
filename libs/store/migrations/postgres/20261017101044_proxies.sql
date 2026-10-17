-- Create "proxies" table
CREATE TABLE "proxies" ("id" uuid NOT NULL, "name" character varying NOT NULL, "name_key" character varying NOT NULL, "kind" character varying NOT NULL, "host" character varying NOT NULL, "port" bigint NOT NULL, "username" character varying NOT NULL DEFAULT '', "password" character varying NOT NULL DEFAULT '', "address_key" character varying NOT NULL, PRIMARY KEY ("id"));
-- Create index "proxies_address_key_key" to table: "proxies"
CREATE UNIQUE INDEX "proxies_address_key_key" ON "proxies" ("address_key");
-- Create index "proxies_name_key_key" to table: "proxies"
CREATE UNIQUE INDEX "proxies_name_key_key" ON "proxies" ("name_key");
