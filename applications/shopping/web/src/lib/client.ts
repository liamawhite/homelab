import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { ItemService } from "../gen/shopping/v1/item_pb";
import { LabelService } from "../gen/shopping/v1/label_pb";

// Same-origin: the Go binary serves both this app and the Connect API on
// one port (see internal/server), so no CORS setup is needed. The Vite dev
// proxy (vite.config.ts) makes this true in development too.
const transport = createConnectTransport({ baseUrl: "/" });

export const itemClient = createClient(ItemService, transport);
export const labelClient = createClient(LabelService, transport);
