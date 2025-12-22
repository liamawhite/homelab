import { certmanager } from './components/kubernetes/certmanager/gen/certmanager'
import { gatewayapi } from './components/kubernetes/istio/gen/gatewayapi'
import { longhorn } from './components/kubernetes/longhorn/gen/longhorn'
import { metallb } from './components/kubernetes/metallb/gen/metallb'
import { prometheusOperator } from './components/kubernetes/prometheus-operator/gen/prometheus-operator'
import { tailscale } from './components/kubernetes/tailscale/gen/tailscale'
;(async () => {
    await Promise.all([
        gatewayapi(),
        certmanager(),
        longhorn(),
        metallb(),
        prometheusOperator(),
        tailscale(),
    ])
})()
