import { createFileRoute } from "@tanstack/react-router";

import { LegalPage } from "../legal.tsx";

export const Route = createFileRoute("/privacy")({ component: () => <LegalPage policy="privacy" /> });
