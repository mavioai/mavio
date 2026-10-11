-- Create "episode_states" table
CREATE TABLE "episode_states" ("id" uuid NOT NULL, "season" bigint NOT NULL, "episode" bigint NOT NULL, "scene_season" bigint NULL, "scene_episode" bigint NULL, "scene_absolute" bigint NULL, "unverified_scene_numbering" boolean NOT NULL DEFAULT false, "last_searched_at" timestamptz NULL, "series_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "episode_states_items_episode_states" FOREIGN KEY ("series_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "episodestate_series_id_season_episode" to table: "episode_states"
CREATE UNIQUE INDEX "episodestate_series_id_season_episode" ON "episode_states" ("series_id", "season", "episode");
