import * as pulumi from '@pulumi/pulumi'
import { remote, local, types } from '@pulumi/command'

export class K3s extends pulumi.ComponentResource {
    readonly kubeconfig: pulumi.Output<string>

    constructor(name: string, args: K3sArgs, opts?: pulumi.ComponentResourceOptions) {
        super('homelab:cluster:k3s', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        if (args.servers.length === 0) {
            throw new Error('At least one server node is required')
        }

        // Temporarily override DNS to avoid issues with DNS resolution when pihole is unavailable i.e. disaster recovery
        const dnsOverrideCmd = 'echo -e "nameserver 1.1.1.1" | sudo tee /etc/resolv.conf'
        const curlCmd = pulumi.interpolate`curl -sfL https://get.k3s.io`
        const serverCmd = pulumi.all([args.sans]).apply(
            ([sans]) => pulumi.interpolate`
            sh -s - server \
                --disable=traefik \
                --disable=servicelb \
                ${sans ? sans.map(san => ` --tls-san ${san}`).join(' ') : ''}`,
        )

        // Cluster initialization first, as we need to know the token for onboarding the remaining servers
        const init = new remote.Command(
            `${name}-init`,
            {
                connection: args.servers[0].connection,
                create: pulumi.interpolate`${dnsOverrideCmd} && ${curlCmd} | ${serverCmd} --cluster-init`,
                update: pulumi.interpolate`${curlCmd} | ${serverCmd}`,
                delete: `sudo /usr/local/bin/k3s-uninstall.sh`,
            },
            { ...localOpts, deleteBeforeReplace: true },
        )

        // Get the cluster token from the initialization step
        const token = pulumi.secret(
            new remote.Command(
                `${name}-token`,
                {
                    connection: args.servers[0].connection,
                    create: `sudo cat /var/lib/rancher/k3s/server/token`,
                },
                { ...localOpts, dependsOn: init },
            ).stdout,
        )

        const servers = args.servers.slice(1).map(
            server =>
                new remote.Command(
                    `${name}-server-${server.name}`,
                    {
                        connection: server.connection,
                        create: pulumi.interpolate`${dnsOverrideCmd} && ${curlCmd} | ${serverCmd} --server https://${args.servers[0].address}:6443 --token ${token}`,
                        delete: `sudo /usr/local/bin/k3s-uninstall.sh`,
                    },
                    {
                        ...localOpts,
                        deleteBeforeReplace: true,
                        dependsOn: [init],
                    },
                ),
        )

        // Retrive the kubeconfig
        const kubeconfigServerIndex = 1 // Which server to retrieve kubeconfig from (0-based index)
        const retrieveKubeconfig = new remote.Command(
            `${name}-kubeconfig`,
            {
                connection: args.servers[kubeconfigServerIndex].connection,
                create: `sudo cat /etc/rancher/k3s/k3s.yaml`,
            },
            { ...localOpts, dependsOn: init },
        ).stdout
        const kubeconfig = pulumi
            .all([args.servers[kubeconfigServerIndex].address, retrieveKubeconfig])
            .apply(([address, config]) => {
                return config.replace('127.0.0.1', address)
            })

        this.kubeconfig = kubeconfig
        this.registerOutputs({ kubeconfig })
    }
}

interface Node extends pulumi.Resource {
    name: string
    address: pulumi.Input<string>
    connection: types.input.remote.ConnectionArgs
}

export interface K3sArgs {
    // All server nodes are also agent nodes
    servers: Node[]

    sans?: pulumi.Input<string>[]
}
