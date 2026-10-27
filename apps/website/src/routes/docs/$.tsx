import { zhCN } from "@fumadocs/language/zh-cn";
import { createFileRoute, notFound } from "@tanstack/react-router";
import { createServerFn } from "@tanstack/react-start";
import { useFumadocsLoader } from "fumadocs-core/source/client";
import { i18nProvider, uiTranslations } from "fumadocs-ui/i18n";
import { DocsLayout } from "fumadocs-ui/layouts/docs";
import { DocsBody, DocsDescription, DocsPage, DocsTitle } from "fumadocs-ui/layouts/docs/page";
import defaultMdxComponents from "fumadocs-ui/mdx";
import { RootProvider } from "fumadocs-ui/provider/tanstack";
import { Suspense, use } from "react";

import { docs, i18n, source } from "../../lib/docs.ts";
import type { Locale } from "../../lib/i18n.ts";
import { useLocale, useT } from "../../lib/locale.tsx";
import { Wordmark } from "../../logo.tsx";

export const Route = createFileRoute("/docs/$")({
  loader: async ({ params, context }) => {
    const data = await page({ data: { slugs: params._splat?.split("/").filter(Boolean) ?? [], lang: context.locale } });
    await docs.getPage(data.path)?.preload();
    return data;
  },
  head: ({ loaderData }) => ({ meta: [{ title: loaderData ? `${loaderData.title} – Mavio` : "Mavio" }] }),
  component: Docs,
});

/** page returns a page of the documentation in a language, and the tree of its pages. */
const page = createServerFn({ method: "GET" })
  .validator((params: { slugs: string[]; lang: Locale }) => params)
  .handler(async ({ data: { slugs, lang } }) => {
    const p = source.getPage(slugs, lang);
    if (!p) throw notFound();
    return { path: p.path, title: p.data.title, pageTree: await source.serializePageTree(source.getPageTree(lang)) };
  });

/** fumadocs' own strings, in the site's languages. */
const translations = i18n.translations().extend(uiTranslations()).preset("zh-Hans", zhCN());

/**
 * The documentation, in fumadocs' layout: the wordmark and the site's links
 * above the tree of pages, the search, and the page with its contents. Its
 * colors follow the system's theme, as the site's.
 */
function Docs() {
  const t = useT();
  const locale = useLocale();
  const data = useFumadocsLoader(Route.useLoaderData());
  return (
    <RootProvider theme={{ enabled: false }} i18n={i18nProvider(translations, locale)}>
      <DocsLayout
        tree={data.pageTree}
        nav={{ title: <Wordmark height={20} />, url: "/" }}
        links={[
          { text: t("site.nav.pass"), url: "/pass" },
          { text: t("site.footer.privacy"), url: "/privacy" },
          { text: t("site.footer.terms"), url: "/terms" },
        ]}
        themeSwitch={{ enabled: false }}
      >
        <Suspense>
          <Content path={data.path} />
        </Suspense>
      </DocsLayout>
    </RootProvider>
  );
}

function Content({ path }: { path: string }) {
  const p = docs.getPage(path);
  if (!p) throw new Error(`unknown page: ${path}`);
  const { toc } = use(p.load());
  const MDX = p.body;
  return (
    <DocsPage toc={toc}>
      <DocsTitle>{p.title}</DocsTitle>
      <DocsDescription>{p.description}</DocsDescription>
      <DocsBody>
        <MDX components={defaultMdxComponents} />
      </DocsBody>
    </DocsPage>
  );
}
