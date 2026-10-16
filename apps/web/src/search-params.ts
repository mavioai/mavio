// The search parameters of the routes, which the router parses as JSON: a
// value that reads as a number or a boolean, such as a remote named 115 in a
// typed address, comes as one.

/** A text search parameter, taken back as text when the router parsed it as a number or a boolean; undefined when empty. */
export function searchText(v: unknown): string | undefined {
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return typeof v === "string" && v !== "" ? v : undefined;
}
