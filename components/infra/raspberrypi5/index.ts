import * as fs from "fs"
import * as path from "path"
import { remote, types } from "@pulumi/command"
import * as pulumi from "@pulumi/pulumi"
import * as checkmate from "@tetratelabs/pulumi-checkmate"

export class RaspberryPi extends pulumi.ComponentResource {
    readonly address: pulumi.Input<string>
    readonly connection: types.input.remote.ConnectionArgs

    constructor(name: string, args: RaspberryPiArgs, opts?: pulumi.ComponentResourceOptions) {
        super("homelab:machine:raspberrypi5", name, {}, opts)
        const localOpts = { ...opts, parent: this }

        this.address = args.connection.host
        this.connection = args.connection

        // Enable NVMe over PCIe
        // Have to use a command because the file is owned by root
        const configTxt = fs.readFileSync(path.join(__dirname, 'config.txt'), 'utf-8')
        const configCopy = new remote.Command(`${name}-config-copy`, {
            connection: args.connection,
            create: `echo "${configTxt}" | sudo tee /boot/firmware/config.txt`,
        }, localOpts)

        // Enable boot from NVMe
        // Have to use a command because the file is owned by root
        const eepromConf = fs.readFileSync(path.join(__dirname, 'eeprom.conf'), 'utf-8')
        const eepromCopy = new remote.Command(`${name}-eeprom-copy`, {
            connection: args.connection,
            create: `echo "${eepromConf}" | sudo tee /boot/firmware/eeprom.conf`,
        }, localOpts)
        const eepromApply = new remote.Command(`${name}-eeprom-apply`, {
            connection: args.connection,
            create: `sudo rpi-eeprom-config --apply /boot/firmware/eeprom.conf`,
        }, { ...localOpts, dependsOn: eepromCopy })

        // Ensure cgroups are enabled for k3s
        const cmdlineTxt = new remote.Command(`${name}-cmdline-txt-read`, {
            connection: args.connection,
            create: `cat /boot/firmware/cmdline.txt`,
        }, localOpts)
        const cmdlineTxtUpdated = cmdlineTxt.stdout.apply(txt => {
            if (!txt.includes('cgroup_memory=1')) {
                txt += ' cgroup_memory=1'
            }
            if (!txt.includes('cgroup_enable=memory')) {
                txt += ' cgroup_enable=memory'
            }
            return txt
        })
        const cmdlineTxtWrite = new remote.Command(`${name}-cmdline-txt-write`, {
            connection: args.connection,
            create: pulumi.interpolate`echo "${cmdlineTxtUpdated}" | sudo tee /boot/firmware/cmdline.txt`,
        }, localOpts)

        // Reboot the machine
        const reboot = new remote.Command(`${name}-reboot`, {
            connection: args.connection,
            create: `sudo reboot`,
        }, { ...localOpts, dependsOn: [configCopy, eepromApply, cmdlineTxtWrite] })

        // Wait for the machine to come back up
        const wait = new checkmate.LocalCommand(`${name}-wait`, {
            command: pulumi.interpolate`ping -c 1 ${args.connection.host}`,
            interval: 500,
            timeout: 1000 * 60 * 5, // give it 5 minutes to come back up
        }, { ...localOpts, dependsOn: reboot })

        this.registerOutputs({ wait: wait.passed })
    }
}


export interface RaspberryPiArgs {
    connection: types.input.remote.ConnectionArgs
}
