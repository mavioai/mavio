// drizzle-kit generates the migrations in drizzle/ from the schema; they
// are applied to the database before a deployment (docs/mavio-dev.md §6).
import { defineConfig } from "drizzle-kit";

export default defineConfig({
  dialect: "postgresql",
  schema: "./src/server/db/schema.ts",
  out: "./drizzle",
  dbCredentials: { url: process.env.DATABASE_URL ?? "" },
});
