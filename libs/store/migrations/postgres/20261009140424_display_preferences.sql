-- Create "display_preferences" table
CREATE TABLE "display_preferences" ("id" uuid NOT NULL, "client" character varying NOT NULL, "view" character varying NOT NULL, "values" jsonb NULL, "updated_at" timestamptz NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "display_preferences_users_display_preferences" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "displaypreferences_user_id_client_view" to table: "display_preferences"
CREATE UNIQUE INDEX "displaypreferences_user_id_client_view" ON "display_preferences" ("user_id", "client", "view");
