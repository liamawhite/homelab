// Every app below is Tailscale-only and reachable directly at
// "<subdomain>.<BASE_DOMAIN>" (the tailnet's MagicDNS suffix) - the
// liamwhite.fyi Cloudflare redirect bookmark now only covers "home" itself
// (pkg/deploy/deploy.go's http_request_dynamic_redirect Ruleset hit its
// 10-rule zone cap), so every other app's card has to point straight at its
// tailnet address. Hardcoded here rather than threaded through config - same
// as every other app's index.html hardcoding its own title and theme color.
export const BASE_DOMAIN = "tailcaa52.ts.net";

export type Group = "apps" | "infra";

export interface AppLink {
  name: string;
  description: string;
  subdomain: string;
  icon: string;
  group: Group;
}

export const GROUP_LABELS: Record<Group, string> = {
  apps: "Apps",
  infra: "Infrastructure",
};

export const APPS: AppLink[] = [
  {
    name: "Workouts",
    description: "Training cycles, blocks, and exercise sets",
    subdomain: "workouts",
    icon: "🏋️",
    group: "apps",
  },
  {
    name: "Shopping",
    description: "Shared shopping list",
    subdomain: "shopping",
    icon: "🛒",
    group: "apps",
  },
  {
    name: "Trips",
    description: "Travel planning and flight tracking",
    subdomain: "trips",
    icon: "✈️",
    group: "apps",
  },
  {
    name: "Reminders",
    description: "One-off reminders",
    subdomain: "reminders",
    icon: "✔️",
    group: "apps",
  },
  {
    name: "Finances",
    description: "Budget and spending tracking",
    subdomain: "finances",
    icon: "💰",
    group: "apps",
  },
  {
    name: "Lights",
    description: "Hue lighting control (lumenetes)",
    subdomain: "lights",
    icon: "💡",
    group: "apps",
  },
  {
    name: "Grafana",
    description: "Dashboards",
    subdomain: "grafana",
    icon: "📊",
    group: "infra",
  },
  {
    name: "Storage",
    description: "Longhorn volumes",
    subdomain: "storage",
    icon: "💾",
    group: "infra",
  },
  {
    name: "Prometheus",
    description: "Metrics",
    subdomain: "prom",
    icon: "🔥",
    group: "infra",
  },
  {
    name: "Alertmanager",
    description: "Alerts",
    subdomain: "alerts",
    icon: "🚨",
    group: "infra",
  },
];

export function appHref(app: AppLink): string {
  return `https://${app.subdomain}.${BASE_DOMAIN}`;
}
