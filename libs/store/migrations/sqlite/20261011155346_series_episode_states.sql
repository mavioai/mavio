-- Create "episode_states" table
CREATE TABLE "episode_states" ("id" uuid NOT NULL, "season" integer NOT NULL, "episode" integer NOT NULL, "scene_season" integer NULL, "scene_episode" integer NULL, "scene_absolute" integer NULL, "unverified_scene_numbering" bool NOT NULL DEFAULT (false), "last_searched_at" datetime NULL, "series_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "episode_states_items_episode_states" FOREIGN KEY ("series_id") REFERENCES "items" ("id") ON DELETE CASCADE);
-- Create index "episodestate_series_id_season_episode" to table: "episode_states"
CREATE UNIQUE INDEX "episodestate_series_id_season_episode" ON "episode_states" ("series_id", "season", "episode");
