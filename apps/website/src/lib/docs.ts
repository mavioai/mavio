// The documentation at /docs (docs/mavio-dev.md §2): MDX pages in
// content/docs, compiled by fumadocs-mdx, in the site's languages, a page's
// translation named <page>.<locale>.mdx beside it.
import { defineI18n } from "fumadocs-core/i18n";
import { loader } from "fumadocs-core/source";
import { defineDocs } from "fumadocs-mdx/macro";

/** The documentation's languages, the site's locales. */
export const i18n = defineI18n({ defaultLanguage: "en", languages: ["en", "zh-Hans"], hideLocale: "always" });

export const docs = defineDocs({ dir: "content/docs", docs: { async: true } });

export const source = loader({ source: docs.toFumadocsSource(), baseUrl: "/docs", i18n });
