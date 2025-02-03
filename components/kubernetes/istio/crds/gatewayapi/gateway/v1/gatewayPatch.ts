// *** WARNING: this file was generated by crd2pulumi. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from '@pulumi/pulumi'
import * as inputs from '../../types/input'
import * as outputs from '../../types/output'
import * as utilities from '../../utilities'

/**
 * Patch resources are used to modify existing Kubernetes resources by using
 * Server-Side Apply updates. The name of the resource must be specified, but all other properties are optional. More than
 * one patch may be applied to the same resource, and a random FieldManager name will be used for each Patch resource.
 * Conflicts will result in an error by default, but can be forced using the "pulumi.com/patchForce" annotation. See the
 * [Server-Side Apply Docs](https://www.pulumi.com/registry/packages/kubernetes/how-to-guides/managing-resources-with-server-side-apply/) for
 * additional information about using Server-Side Apply to manage Kubernetes resources with Pulumi.
 * Gateway represents an instance of a service-traffic handling infrastructure
 * by binding Listeners to a set of IP addresses.
 */
export class GatewayPatch extends pulumi.CustomResource {
    /**
     * Get an existing GatewayPatch resource's state with the given name, ID, and optional extra
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
    ): GatewayPatch {
        return new GatewayPatch(name, undefined as any, { ...opts, id: id })
    }

    /** @internal */
    public static readonly __pulumiType = 'kubernetes:gateway.networking.k8s.io/v1:GatewayPatch'

    /**
     * Returns true if the given object is an instance of GatewayPatch.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is GatewayPatch {
        if (obj === undefined || obj === null) {
            return false
        }
        return obj['__pulumiType'] === GatewayPatch.__pulumiType
    }

    /**
     * APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
     */
    public readonly apiVersion!: pulumi.Output<'gateway.networking.k8s.io/v1'>
    /**
     * Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
     */
    public readonly kind!: pulumi.Output<'Gateway'>
    /**
     * Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
     */
    public readonly metadata!: pulumi.Output<outputs.meta.v1.ObjectMetaPatch>
    public readonly spec!: pulumi.Output<outputs.gateway.v1.GatewaySpecPatch>
    public readonly /*out*/ status!: pulumi.Output<outputs.gateway.v1.GatewayStatusPatch>

    /**
     * Create a GatewayPatch resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args?: GatewayPatchArgs, opts?: pulumi.CustomResourceOptions) {
        let resourceInputs: pulumi.Inputs = {}
        opts = opts || {}
        if (!opts.id) {
            resourceInputs['apiVersion'] = 'gateway.networking.k8s.io/v1'
            resourceInputs['kind'] = 'Gateway'
            resourceInputs['metadata'] = args ? args.metadata : undefined
            resourceInputs['spec'] = args ? args.spec : undefined
            resourceInputs['status'] = undefined /*out*/
        } else {
            resourceInputs['apiVersion'] = undefined /*out*/
            resourceInputs['kind'] = undefined /*out*/
            resourceInputs['metadata'] = undefined /*out*/
            resourceInputs['spec'] = undefined /*out*/
            resourceInputs['status'] = undefined /*out*/
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts)
        const aliasOpts = {
            aliases: [{ type: 'kubernetes:gateway.networking.k8s.io/v1beta1:GatewayPatch' }],
        }
        opts = pulumi.mergeOptions(opts, aliasOpts)
        super(GatewayPatch.__pulumiType, name, resourceInputs, opts)
    }
}

/**
 * The set of arguments for constructing a GatewayPatch resource.
 */
export interface GatewayPatchArgs {
    /**
     * APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
     */
    apiVersion?: pulumi.Input<'gateway.networking.k8s.io/v1'>
    /**
     * Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
     */
    kind?: pulumi.Input<'Gateway'>
    /**
     * Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
     */
    metadata?: pulumi.Input<inputs.meta.v1.ObjectMetaPatch>
    spec?: pulumi.Input<inputs.gateway.v1.GatewaySpecPatch>
}
