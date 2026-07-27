import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { BridgeService } from "../gen/lumenetes/v1/bridge_pb";
import { LightService } from "../gen/lumenetes/v1/light_pb";
import { SwitchService } from "../gen/lumenetes/v1/switch_pb";
import { GroupService } from "../gen/lumenetes/v1/group_pb";
import { SceneService } from "../gen/lumenetes/v1/scene_pb";
import { CircadianScheduleService } from "../gen/lumenetes/v1/circadian_schedule_pb";

// Same-origin: the Go binary serves both this app and the Connect API on
// one port (see internal/server), so no CORS setup is needed. The Vite dev
// proxy (vite.config.ts) makes this true in development too.
const transport = createConnectTransport({ baseUrl: "/" });

export const bridgeClient = createClient(BridgeService, transport);
export const lightClient = createClient(LightService, transport);
export const switchClient = createClient(SwitchService, transport);
export const groupClient = createClient(GroupService, transport);
export const sceneClient = createClient(SceneService, transport);
export const circadianScheduleClient = createClient(CircadianScheduleService, transport);
