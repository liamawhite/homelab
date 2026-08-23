import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { UserService } from "../gen/finances/v1/user_pb";
import { AssetService } from "../gen/finances/v1/asset_pb";
import { ExchangeRateService } from "../gen/finances/v1/exchange_rate_pb";

// Same-origin: the Go binary serves both this app and the Connect API on
// one port (see internal/server), so no CORS setup is needed. The Vite dev
// proxy (vite.config.ts) makes this true in development too.
const transport = createConnectTransport({ baseUrl: "/" });

export const userClient = createClient(UserService, transport);
export const assetClient = createClient(AssetService, transport);
export const exchangeRateClient = createClient(ExchangeRateService, transport);
