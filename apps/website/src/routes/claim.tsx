import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { accounts } from "../lib/api.ts";
import { describe, useLocale, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Block, Button, ErrorText, Group, Loading, Muted, Page } from "../ui.tsx";

// A claim code (docs/accounts.md §3.2) for a server started without a
// browser at hand, in MAVIO_CLAIM_TOKEN; a server set up in a browser is
// claimed by signing in to it (/authorize).
export const Route = createFileRoute("/claim")({ component: Claim });

function Claim() {
  const t = useT();
  const locale = useLocale();
  const me = useAccount();
  const code = useMutation({ mutationFn: () => accounts.createClaimCode({}) });
  if (!me) return <Loading />;
  const expires = code.data?.expireTime && timestampDate(code.data.expireTime);
  return (
    <Page title={t("site.nav.claim")}>
      <Group>
        <Block>
          <Muted>{t("site.claim.body")}</Muted>
          {code.data ? (
            <div className="flex flex-col items-center gap-2 rounded-[12px] bg-fill px-4 py-6">
              <p className="font-mono text-[34px] leading-10 font-semibold tracking-[0.2em] select-all">{code.data.claimCode}</p>
              <p className="type-footnote text-text-secondary">
                {t("site.claim.valid", { time: expires?.toLocaleTimeString(locale, { hour: "numeric", minute: "2-digit" }) ?? "" })}
              </p>
            </div>
          ) : (
            <div>
              <Button onClick={() => code.mutate()} disabled={code.isPending}>
                {t("site.claim.get")}
              </Button>
            </div>
          )}
          <ErrorText error={describe(t, code.error)} />
        </Block>
      </Group>
    </Page>
  );
}
