-- Create "display_preferences" table
CREATE TABLE "display_preferences" ("id" uuid NOT NULL, "client" text NOT NULL, "view" text NOT NULL, "values" json NULL, "updated_at" datetime NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "display_preferences_users_display_preferences" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
-- Create index "displaypreferences_user_id_client_view" to table: "display_preferences"
CREATE UNIQUE INDEX "displaypreferences_user_id_client_view" ON "display_preferences" ("user_id", "client", "view");
