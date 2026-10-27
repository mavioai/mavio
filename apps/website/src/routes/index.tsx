import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { PosterWall } from "../auth-ui.tsx";
import { accounts } from "../lib/api.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { useAccount, useSignedIn } from "../lib/session.ts";
import { Button, Empty, ErrorText, Group, Icon, type IconName, Loading, Name, Page, Row } from "../ui.tsx";

export const Route = createFileRoute("/")({ component: Index });

/** The front page: the account's servers when signed in, what Mavio is when not. */
function Index() {
  return useSignedIn() ? <Servers /> : <Landing />;
}

const features: [IconName, "own" | "library" | "screens" | "anywhere" | "home" | "play"][] = [
  ["server", "own"],
  ["collections", "library"],
  ["devices", "screens"],
  ["cloud", "anywhere"],
  ["users", "home"],
  ["play", "play"],
];

/** What Mavio is, for visitors: the headline on the wall of posters, the features and Mavio Pass. */
function Landing() {
  const t = useT();
  return (
    <main className="flex flex-col">
      <section className="relative overflow-hidden">
        <PosterWall />
        <div className="relative mx-auto flex max-w-[1080px] flex-col items-start gap-6 px-margin py-24 sm:py-32">
          <h1 className="max-w-[640px] text-[40px] leading-[46px] font-bold tracking-[-0.02em] sm:text-[56px] sm:leading-[62px]">{t("site.landing.title")}</h1>
          <p className="max-w-[560px] text-[17px] leading-[26px] text-text-secondary sm:text-[19px] sm:leading-[28px]">{t("site.landing.subtitle")}</p>
          <div className="flex flex-wrap gap-3">
            <Link to="/sign-up" className="inline-flex min-h-12 items-center rounded-[12px] bg-button px-6 text-[16px] font-semibold text-on-button transition hover:opacity-90">
              {t("site.landing.start")}
            </Link>
            <a href="/docs" className="inline-flex min-h-12 items-center rounded-[12px] bg-fill px-6 text-[16px] font-semibold text-on-background backdrop-blur-xl transition hover:bg-fill-selected">
              {t("site.landing.docs")}
            </a>
          </div>
        </div>
      </section>
      <section className="mx-auto flex w-full max-w-[1080px] flex-col gap-6 px-margin py-16">
        <h2 className="type-title1">{t("site.landing.features")}</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map(([icon, key]) => (
            <div key={key} className="flex flex-col gap-3 rounded-[14px] bg-surface p-5">
              <span className="flex size-10 items-center justify-center rounded-[10px] bg-fill text-accent">
                <Icon name={icon} size={22} />
              </span>
              <h3 className="type-headline">{t(`site.landing.${key}.title`)}</h3>
              <p className="type-callout text-text-secondary">{t(`site.landing.${key}.body`)}</p>
            </div>
          ))}
        </div>
      </section>
      <section className="mx-auto w-full max-w-[1080px] px-margin pb-20">
        <div className="flex flex-wrap items-center gap-x-8 gap-y-4 rounded-[18px] bg-surface p-6 sm:p-8">
          <Icon name="key" size={32} className="text-accent" />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <h2 className="type-title3">{t("site.nav.pass")}</h2>
            <p className="max-w-[640px] type-callout text-text-secondary">{t("site.landing.pass.body")}</p>
          </div>
          <Link to="/pass" className="inline-flex min-h-9 items-center rounded-md bg-fill px-4 type-headline text-on-background hover:bg-fill-selected">
            {t("site.landing.pass.more")}
          </Link>
        </div>
      </section>
    </main>
  );
}

function Servers() {
  const t = useT();
  const rich = useRich();
  const me = useAccount();
  const qc = useQueryClient();
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}), enabled: !!me });
  const invitations = useQuery({ queryKey: ["invitations"], queryFn: () => accounts.listInvitations({}), enabled: !!me });
  const answer = useMutation({
    mutationFn: async ({ serverId, accept }: { serverId: string; accept: boolean }) => {
      if (accept) await accounts.acceptInvitation({ serverId });
      else await accounts.declineInvitation({ serverId });
    },
    onSuccess: () => qc.invalidateQueries(),
  });
  if (!me || servers.isPending) return <Loading />;

  const pending = invitations.data?.invitations ?? [];
  return (
    <Page title={t("site.nav.servers")}>
      {pending.length > 0 && (
        <Group title={t("site.invitations.title")} footer={<ErrorText error={describe(t, answer.error)} />}>
          {pending.map((i) => (
            <Row
              key={i.serverId}
              leading={<Icon name="server" size={28} className="text-accent" />}
              label={rich("site.invitations.server", {
                name: i.administrator?.displayName || i.administrator?.userName,
                server: <Name>{i.serverName || i.serverId}</Name>,
              })}
            >
              <Button onClick={() => answer.mutate({ serverId: i.serverId, accept: true })}>{t("site.action.accept")}</Button>
              <Button variant="secondary" onClick={() => answer.mutate({ serverId: i.serverId, accept: false })}>
                {t("site.action.decline")}
              </Button>
            </Row>
          ))}
        </Group>
      )}
      <ErrorText error={describe(t, servers.error)} />
      {servers.data?.servers.length === 0 ? (
        <Empty icon="server" title={t("site.servers.empty")} message={t("site.servers.emptyBody")}>
          <Link to="/claim" className="mt-2 inline-flex min-h-9 items-center rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85">
            {t("site.nav.claim")}
          </Link>
        </Empty>
      ) : (
        servers.data && (
          <Group>
            {servers.data.servers.map((s) => (
              <Link key={s.id} to="/servers/$id" params={{ id: s.id }} className="transition-colors hover:bg-fill">
                <Row
                  leading={
                    <span className="flex size-10 items-center justify-center rounded-[10px] bg-fill text-accent">
                      <Icon name="server" size={22} />
                    </span>
                  }
                  label={<span className="font-semibold">{s.name || s.id}</span>}
                  detail={[
                    s.administrator?.id === me.id ? t("site.servers.administer") : t("site.servers.administeredBy", { name: s.administrator?.displayName || s.administrator?.userName || "" }),
                    t("site.servers.libraries", { count: s.libraries.length }),
                  ].join(" · ")}
                >
                  <Icon name="forward" size={18} className="text-text-tertiary" />
                </Row>
              </Link>
            ))}
          </Group>
        )
      )}
    </Page>
  );
}
