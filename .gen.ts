import { certmanager } from "./components/kubernetes/certmanager/gen/certmanager";
import { gatewayapi } from "./components/kubernetes/istio/gen/gatewayapi";

(async () => {
    await Promise.all([
        gatewayapi(),
        certmanager(),
    ])
})()

