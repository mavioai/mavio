// The better-auth instance the auth command line reads to generate
// src/server/db/auth-schema.ts; it never connects to a database.
import { createAuth } from "../src/server/auth.ts";
import type { Database } from "../src/server/db/index.ts";

export const auth = createAuth({} as Database, { baseURL: "https://mavio.dev", secret: "generate" });
