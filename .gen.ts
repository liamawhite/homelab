import { certmanager } from './components/kubernetes/certmanager/gen/certmanager'
import { gatewayapi } from './components/kubernetes/istio/gen/gatewayapi'
import { metallb } from './components/kubernetes/metallb/gen/metallb'
import { tailscale } from './components/kubernetes/tailscale/gen/tailscale'
;(async () => {
    await Promise.all([gatewayapi(), certmanager(), metallb(), tailscale()])
})()
