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
 * BFDProfile represents the settings of the bfd session that can be
 * optionally associated with a BGP session.
 */
export class BFDProfilePatch extends pulumi.CustomResource {
    /**
     * Get an existing BFDProfilePatch resource's state with the given name, ID, and optional extra
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
    ): BFDProfilePatch {
        return new BFDProfilePatch(name, undefined as any, { ...opts, id: id })
    }

    /** @internal */
    public static readonly __pulumiType =
        'kubernetes:metallb.io/v1beta1:BFDProfilePatch'

    /**
     * Returns true if the given object is an instance of BFDProfilePatch.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is BFDProfilePatch {
        if (obj === undefined || obj === null) {
            return false
        }
        return obj['__pulumiType'] === BFDProfilePatch.__pulumiType
    }

    /**
     * APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
     */
    public readonly apiVersion!: pulumi.Output<'metallb.io/v1beta1'>
    /**
     * Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
     */
    public readonly kind!: pulumi.Output<'BFDProfile'>
    /**
     * Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
     */
    public readonly metadata!: pulumi.Output<outputs.meta.v1.ObjectMetaPatch>
    public readonly spec!: pulumi.Output<outputs.metallb.v1beta1.BFDProfileSpecPatch>
    /**
     * BFDProfileStatus defines the observed state of BFDProfile.
     */
    public readonly /*out*/ status!: pulumi.Output<{ [key: string]: string }>

    /**
     * Create a BFDProfilePatch resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(
        name: string,
        args?: BFDProfilePatchArgs,
        opts?: pulumi.CustomResourceOptions,
    ) {
        let resourceInputs: pulumi.Inputs = {}
        opts = opts || {}
        if (!opts.id) {
            resourceInputs['apiVersion'] = 'metallb.io/v1beta1'
            resourceInputs['kind'] = 'BFDProfile'
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
        super(BFDProfilePatch.__pulumiType, name, resourceInputs, opts)
    }
}

/**
 * The set of arguments for constructing a BFDProfilePatch resource.
 */
export interface BFDProfilePatchArgs {
    /**
     * APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
     */
    apiVersion?: pulumi.Input<'metallb.io/v1beta1'>
    /**
     * Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
     */
    kind?: pulumi.Input<'BFDProfile'>
    /**
     * Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
     */
    metadata?: pulumi.Input<inputs.meta.v1.ObjectMetaPatch>
    spec?: pulumi.Input<inputs.metallb.v1beta1.BFDProfileSpecPatch>
}
