// *** WARNING: this file was generated by crd2pulumi. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from '@pulumi/pulumi'
import * as inputs from '../../types/input'
import * as outputs from '../../types/output'
import * as utilities from '../../utilities'

import { ObjectMeta } from '../../meta/v1'

export class Recorder extends pulumi.CustomResource {
    /**
     * Get an existing Recorder resource's state with the given name, ID, and optional extra
     * properties used to qualify the lookup.
     *
     * @param name The _unique_ name of the resulting resource.
     * @param id The _unique_ provider ID of the resource to lookup.
     * @param opts Optional settings to control the behavior of the CustomResource.
     */
    public static get(
        name: string,
        id: pulumi.Input<pulumi.ID>,
        opts?: pulumi.CustomResourceOptions,
    ): Recorder {
        return new Recorder(name, undefined as any, { ...opts, id: id })
    }

    /** @internal */
    public static readonly __pulumiType = 'kubernetes:tailscale.com/v1alpha1:Recorder'

    /**
     * Returns true if the given object is an instance of Recorder.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is Recorder {
        if (obj === undefined || obj === null) {
            return false
        }
        return obj['__pulumiType'] === Recorder.__pulumiType
    }

    public readonly apiVersion!: pulumi.Output<'tailscale.com/v1alpha1' | undefined>
    public readonly kind!: pulumi.Output<'Recorder' | undefined>
    public readonly metadata!: pulumi.Output<ObjectMeta | undefined>
    /**
     * Spec describes the desired recorder instance.
     */
    public readonly spec!: pulumi.Output<outputs.tailscale.v1alpha1.RecorderSpec>
    /**
     * RecorderStatus describes the status of the recorder. This is set
     * and managed by the Tailscale operator.
     */
    public readonly status!: pulumi.Output<outputs.tailscale.v1alpha1.RecorderStatus | undefined>

    /**
     * Create a Recorder resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args?: RecorderArgs, opts?: pulumi.CustomResourceOptions) {
        let resourceInputs: pulumi.Inputs = {}
        opts = opts || {}
        if (!opts.id) {
            resourceInputs['apiVersion'] = 'tailscale.com/v1alpha1'
            resourceInputs['kind'] = 'Recorder'
            resourceInputs['metadata'] = args ? args.metadata : undefined
            resourceInputs['spec'] = args ? args.spec : undefined
            resourceInputs['status'] = args ? args.status : undefined
        } else {
            resourceInputs['apiVersion'] = undefined /*out*/
            resourceInputs['kind'] = undefined /*out*/
            resourceInputs['metadata'] = undefined /*out*/
            resourceInputs['spec'] = undefined /*out*/
            resourceInputs['status'] = undefined /*out*/
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts)
        super(Recorder.__pulumiType, name, resourceInputs, opts)
    }
}

/**
 * The set of arguments for constructing a Recorder resource.
 */
export interface RecorderArgs {
    apiVersion?: pulumi.Input<'tailscale.com/v1alpha1'>
    kind?: pulumi.Input<'Recorder'>
    metadata?: pulumi.Input<ObjectMeta>
    /**
     * Spec describes the desired recorder instance.
     */
    spec?: pulumi.Input<inputs.tailscale.v1alpha1.RecorderSpecArgs>
    /**
     * RecorderStatus describes the status of the recorder. This is set
     * and managed by the Tailscale operator.
     */
    status?: pulumi.Input<inputs.tailscale.v1alpha1.RecorderStatusArgs>
}
