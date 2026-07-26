import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { UserService } from "../gen/workouts/v1/user_pb";
import { ExerciseService } from "../gen/workouts/v1/exercise_pb";
import { TrainingMaxService } from "../gen/workouts/v1/training_max_pb";

// Same-origin: the Go binary serves both this app and the Connect API on
// one port (see internal/server), so no CORS setup is needed. The Vite dev
// proxy (vite.config.ts) makes this true in development too.
const transport = createConnectTransport({ baseUrl: "/" });

export const userClient = createClient(UserService, transport);
export const exerciseClient = createClient(ExerciseService, transport);
export const trainingMaxClient = createClient(TrainingMaxService, transport);
