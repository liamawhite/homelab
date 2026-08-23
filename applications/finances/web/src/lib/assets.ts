import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { MessageInitShape } from "@bufbuild/protobuf";

import { assetClient } from "./client";
import type { Asset } from "@/gen/finances/v1/asset_pb";
import { CreateAssetRequestSchema, UpdateAssetRequestSchema } from "@/gen/finances/v1/asset_pb";

const assetsQueryKey = ["assets"];
export const valuationsQueryKey = (assetId: string) => ["assets", assetId, "valuations"];

// The name people actually recognize an asset by - shared between the
// assets table and anywhere else (e.g. the exposure chart's hover
// breakdown) that needs to name an asset without duplicating this switch.
export function assetDisplayName(asset: Asset): string {
  switch (asset.details.case) {
    case "stockOrBond":
      return asset.details.value.companyOrBondName;
    case "cash":
      return asset.details.value.accountName;
    case "etf":
      return asset.details.value.etfName;
    case "retirement":
      return asset.details.value.planName;
    case "realEstate":
      return asset.details.value.propertyName;
    case "loan":
      return asset.details.value.name;
    case "mortgage":
      return asset.details.value.propertyName;
    case "capitalBalance":
      return asset.details.value.name;
    default:
      return "—";
  }
}

export function useAssets() {
  return useQuery({
    queryKey: assetsQueryKey,
    queryFn: async () => (await assetClient.listAssets({})).assets,
  });
}

export function useCreateAsset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof CreateAssetRequestSchema>) => {
      const res = await assetClient.createAsset(input);
      if (!res.asset) {
        throw new Error("CreateAsset response did not include an asset");
      }
      return res.asset;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: assetsQueryKey });
    },
  });
}

export function useUpdateAsset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof UpdateAssetRequestSchema>) => {
      const res = await assetClient.updateAsset(input);
      if (!res.asset) {
        throw new Error("UpdateAsset response did not include an asset");
      }
      return res.asset;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: assetsQueryKey });
    },
  });
}

export function useDeleteAsset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await assetClient.deleteAsset({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: assetsQueryKey });
    },
  });
}

export function useAssetValuations(assetId: string) {
  return useQuery({
    queryKey: valuationsQueryKey(assetId),
    queryFn: async () => (await assetClient.listAssetValuations({ assetId })).valuations,
    enabled: !!assetId,
  });
}

export function useAddAssetValuation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { assetId: string; date: string; amount: bigint }) => {
      const res = await assetClient.addAssetValuation(input);
      if (!res.valuation) {
        throw new Error("AddAssetValuation response did not include a valuation");
      }
      return res.valuation;
    },
    onSuccess: (_valuation, variables) => {
      void queryClient.invalidateQueries({ queryKey: assetsQueryKey });
      void queryClient.invalidateQueries({ queryKey: valuationsQueryKey(variables.assetId) });
    },
  });
}

export function useDeleteAssetValuation() {
  const queryClient = useQueryClient();

  return useMutation({
    // assetId isn't part of the API call - DeleteAssetValuation only needs
    // the valuation's own id - it's carried through so onSuccess knows
    // which valuations list to invalidate.
    mutationFn: async (input: { valuationId: string; assetId: string }) => {
      await assetClient.deleteAssetValuation({ valuationId: input.valuationId });
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: assetsQueryKey });
      void queryClient.invalidateQueries({ queryKey: valuationsQueryKey(variables.assetId) });
    },
  });
}

// Assets are stored server-side as int64 minor units (cents/pence) to avoid
// float rounding on money - these convert at the form boundary, where the
// user thinks in decimal dollars/pounds.
export function toMinorUnits(amount: number): bigint {
  return BigInt(Math.round(amount * 100));
}

export function fromMinorUnits(minor: bigint): number {
  return Number(minor) / 100;
}
