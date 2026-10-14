import type { ExtractTablesWithRelations } from "drizzle-orm";
import type { PgDatabase, PgQueryResultHKT, PgTransaction } from "drizzle-orm/pg-core";

import * as schema from "./schema.ts";

export { schema };

type Schema = typeof schema;

/** Database is mavio.dev's database through Drizzle, whichever the driver. */
export type Database = PgDatabase<PgQueryResultHKT, Schema>;

/** Tx is a transaction of Database. */
export type Tx = PgTransaction<PgQueryResultHKT, Schema, ExtractTablesWithRelations<Schema>>;
