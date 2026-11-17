// Migrates an empty database once for the whole run, before any test file starts, so that no test's timeout
// includes the migration however busy the machine is.
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { PGlite } from "@electric-sql/pglite";
import { drizzle } from "drizzle-orm/pglite";
import { migrate } from "drizzle-orm/pglite/migrator";
import type { TestProject } from "vitest/node";

declare module "vitest" {
  export interface ProvidedContext {
    /** The path of the data directory of an empty, migrated database, as PGlite dumps it. */
    database: string;
  }
}

export default async function setup(project: TestProject) {
  const pg = new PGlite();
  await migrate(drizzle(pg), { migrationsFolder: fileURLToPath(new URL("../drizzle", import.meta.url)) });
  const dump = await pg.dumpDataDir("none");
  await pg.close();
  const dir = await mkdtemp(join(tmpdir(), "mavio-website-"));
  const file = join(dir, "database.tar");
  await writeFile(file, new Uint8Array(await dump.arrayBuffer()));
  project.provide("database", file);
  return () => rm(dir, { recursive: true, force: true });
}
