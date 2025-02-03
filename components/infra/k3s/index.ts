import * as pulumi from '@pulumi/pulumi'
import { remote, types } from '@pulumi/command'

export class K3s extends pulumi.ComponentResource {
    readonly kubeconfig: pulumi.Output<string>

    constructor(
        name: string,
        args: K3sArgs,
        opts?: pulumi.ComponentResourceOptions,
    ) {
        super('homelab:cluster:k3s', name, {}, opts)
        const localOpts = { ...opts, parent: this }

        if (args.servers.length === 0) {
            throw new Error('At least one server node is required')
        }

        // Build the server command
        const serverCmd = pulumi
            .all([args.clusterToken, args.sans])
            .apply(([clusterToken, sans]) => {
                let cmd = 'curl -sfL https://get.k3s.io | sh -s - server'
                cmd += ' --disable=traefik'
                cmd += ' --disable=servicelb'
                cmd += ' --disable=local-storage'
                cmd += ` --token ${clusterToken}`
                if (sans) {
                    sans.forEach(san => (cmd += ` --tls-san ${san}`))
                }
                return cmd
            })

        // There are probably gremlins here if commands are updated?
        const cmds = args.servers.reduce((acc, server, i) => {
            const curr = new remote.Command(
                `${name}-server-${i}`,
                {
                    connection: server.connection,
                    create:
                        i === 0
                            ? pulumi.interpolate`${serverCmd} --cluster-init`
                            : pulumi.interpolate`${serverCmd} --server https://${args.servers[0].address}:6443`,
                    update: serverCmd,
                    delete: `sudo /usr/local/bin/k3s-uninstall.sh`,
                },
                {
                    ...localOpts,
                    deleteBeforeReplace: true,
                    dependsOn: [...acc, server],
                },
            )
            return [...acc, curr]
        }, [] as remote.Command[])

        // Retrive the kubeconfig
        const retrieveKubeconfig = new remote.Command(
            `${name}-kubeconfig`,
            {
                connection: args.servers[0].connection,
                create: `sudo cat /etc/rancher/k3s/k3s.yaml`,
            },
            { ...localOpts, dependsOn: cmds },
        ).stdout
        const kubeconfig = pulumi
            .all([args.servers[0].address, retrieveKubeconfig])
            .apply(([address, config]) => {
                return config.replace('127.0.0.1', address)
            })

        this.kubeconfig = kubeconfig
        this.registerOutputs({ kubeconfig })
    }
}

interface Node extends pulumi.Resource {
    address: pulumi.Input<string>
    connection: types.input.remote.ConnectionArgs
}

export interface K3sArgs {
    // All server nodes are also agent nodes
    servers: Node[]

    clusterToken: pulumi.Input<string>
    sans?: pulumi.Input<string>[]
}
