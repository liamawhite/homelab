// *** WARNING: this file was generated by crd2pulumi. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as inputs from "../../types/input";
import * as outputs from "../../types/output";
import * as utilities from "../../utilities";

import {ObjectMeta} from "../../meta/v1";

export class ProxyGroup extends pulumi.CustomResource {
    /**
     * Get an existing ProxyGroup resource's state with the given name, ID, and optional extra
     * properties used to qualify the lookup.
     *
     * @param name The _unique_ name of the resulting resource.
     * @param id The _unique_ provider ID of the resource to lookup.
     * @param opts Optional settings to control the behavior of the CustomResource.
     */
    public static get(name: string, id: pulumi.Input<pulumi.ID>, opts?: pulumi.CustomResourceOptions): ProxyGroup {
        return new ProxyGroup(name, undefined as any, { ...opts, id: id });
    }

    /** @internal */
    public static readonly __pulumiType = 'kubernetes:tailscale.com/v1alpha1:ProxyGroup';

    /**
     * Returns true if the given object is an instance of ProxyGroup.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is ProxyGroup {
        if (obj === undefined || obj === null) {
            return false;
        }
        return obj['__pulumiType'] === ProxyGroup.__pulumiType;
    }

    public readonly apiVersion!: pulumi.Output<"tailscale.com/v1alpha1" | undefined>;
    public readonly kind!: pulumi.Output<"ProxyGroup" | undefined>;
    public readonly metadata!: pulumi.Output<ObjectMeta | undefined>;
    /**
     * Spec describes the desired ProxyGroup instances.
     */
    public readonly spec!: pulumi.Output<outputs.tailscale.v1alpha1.ProxyGroupSpec>;
    /**
     * ProxyGroupStatus describes the status of the ProxyGroup resources. This is
     * set and managed by the Tailscale operator.
     */
    public readonly status!: pulumi.Output<outputs.tailscale.v1alpha1.ProxyGroupStatus | undefined>;

    /**
     * Create a ProxyGroup resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args?: ProxyGroupArgs, opts?: pulumi.CustomResourceOptions) {
        let resourceInputs: pulumi.Inputs = {};
        opts = opts || {};
        if (!opts.id) {
            resourceInputs["apiVersion"] = "tailscale.com/v1alpha1";
            resourceInputs["kind"] = "ProxyGroup";
            resourceInputs["metadata"] = args ? args.metadata : undefined;
            resourceInputs["spec"] = args ? args.spec : undefined;
            resourceInputs["status"] = args ? args.status : undefined;
        } else {
            resourceInputs["apiVersion"] = undefined /*out*/;
            resourceInputs["kind"] = undefined /*out*/;
            resourceInputs["metadata"] = undefined /*out*/;
            resourceInputs["spec"] = undefined /*out*/;
            resourceInputs["status"] = undefined /*out*/;
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts);
        super(ProxyGroup.__pulumiType, name, resourceInputs, opts);
    }
}

/**
 * The set of arguments for constructing a ProxyGroup resource.
 */
export interface ProxyGroupArgs {
    apiVersion?: pulumi.Input<"tailscale.com/v1alpha1">;
    kind?: pulumi.Input<"ProxyGroup">;
    metadata?: pulumi.Input<ObjectMeta>;
    /**
     * Spec describes the desired ProxyGroup instances.
     */
    spec?: pulumi.Input<inputs.tailscale.v1alpha1.ProxyGroupSpecArgs>;
    /**
     * ProxyGroupStatus describes the status of the ProxyGroup resources. This is
     * set and managed by the Tailscale operator.
     */
    status?: pulumi.Input<inputs.tailscale.v1alpha1.ProxyGroupStatusArgs>;
}
