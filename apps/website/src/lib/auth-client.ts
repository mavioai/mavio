// better-auth's client for the website's pages.
import { passkeyClient } from "@better-auth/passkey/client";
import { deviceAuthorizationClient, twoFactorClient, usernameClient } from "better-auth/client/plugins";
import { createAuthClient } from "better-auth/react";

export const authClient = createAuthClient({
  basePath: "/api/auth",
  plugins: [
    usernameClient(),
    passkeyClient(),
    deviceAuthorizationClient(),
    twoFactorClient({
      onTwoFactorRedirect: () => {
        window.location.href = "/sign-in?step=totp";
      },
    }),
  ],
});
