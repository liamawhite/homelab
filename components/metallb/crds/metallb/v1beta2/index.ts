// *** WARNING: this file was generated by crd2pulumi. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as utilities from "../../utilities";

// Export members:
export { BGPPeerArgs } from "./bgppeer";
export type BGPPeer = import("./bgppeer").BGPPeer;
export const BGPPeer: typeof import("./bgppeer").BGPPeer = null as any;
utilities.lazyLoad(exports, ["BGPPeer"], () => require("./bgppeer"));

export { BGPPeerListArgs } from "./bgppeerList";
export type BGPPeerList = import("./bgppeerList").BGPPeerList;
export const BGPPeerList: typeof import("./bgppeerList").BGPPeerList = null as any;
utilities.lazyLoad(exports, ["BGPPeerList"], () => require("./bgppeerList"));

export { BGPPeerPatchArgs } from "./bgppeerPatch";
export type BGPPeerPatch = import("./bgppeerPatch").BGPPeerPatch;
export const BGPPeerPatch: typeof import("./bgppeerPatch").BGPPeerPatch = null as any;
utilities.lazyLoad(exports, ["BGPPeerPatch"], () => require("./bgppeerPatch"));


const _module = {
    version: utilities.getVersion(),
    construct: (name: string, type: string, urn: string): pulumi.Resource => {
        switch (type) {
            case "kubernetes:metallb.io/v1beta2:BGPPeer":
                return new BGPPeer(name, <any>undefined, { urn })
            case "kubernetes:metallb.io/v1beta2:BGPPeerList":
                return new BGPPeerList(name, <any>undefined, { urn })
            case "kubernetes:metallb.io/v1beta2:BGPPeerPatch":
                return new BGPPeerPatch(name, <any>undefined, { urn })
            default:
                throw new Error(`unknown resource type ${type}`);
        }
    },
};
pulumi.runtime.registerResourceModule("crds", "metallb.io/v1beta2", _module)
