import { useMemo, useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { History, Pencil, Plus, Trash2 } from "lucide-react";

import type { Asset, AssetValuation } from "@/gen/finances/v1/asset_pb";
import { AssetCategory, AssetLiquidity, Currency, Region, SecurityType } from "@/gen/finances/v1/asset_pb";
import {
  useAddAssetValuation,
  useAssets,
  useAssetValuations,
  useCreateAsset,
  useDeleteAsset,
  useDeleteAssetValuation,
  useUpdateAsset,
  assetDisplayName,
  fromMinorUnits,
  toMinorUnits,
} from "@/lib/assets";
import { useUsers } from "@/lib/users";
import { formatDate } from "@/lib/dates";
import { useLiquidityFilter, liquidityMatches } from "@/lib/liquidityFilter";
import { useDisplayCurrency } from "@/lib/displayCurrency";
import { useExchangeRates, convertAmount } from "@/lib/exchangeRates";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const CATEGORY_LABELS: Record<AssetCategory, string> = {
  [AssetCategory.UNSPECIFIED]: "Unknown",
  [AssetCategory.STOCK_OR_BOND]: "Individual Stock or Bond",
  [AssetCategory.CASH]: "Cash",
  [AssetCategory.ETF]: "ETF",
  [AssetCategory.RETIREMENT]: "Retirement",
  [AssetCategory.REAL_ESTATE]: "Real Estate",
  [AssetCategory.LOAN]: "Loan",
  [AssetCategory.MORTGAGE]: "Mortgage",
  [AssetCategory.CAPITAL_BALANCE]: "Capital Balance",
};

const REGION_LABELS: Record<Region, string> = {
  [Region.UNSPECIFIED]: "Unknown",
  [Region.US]: "US",
  [Region.INTERNATIONAL]: "International",
  [Region.UK]: "UK",
  [Region.BONDS]: "Bonds",
};

const SECURITY_TYPE_LABELS: Record<SecurityType, string> = {
  [SecurityType.UNSPECIFIED]: "Unknown",
  [SecurityType.STOCK]: "Stock",
  [SecurityType.BOND]: "Bond",
  [SecurityType.COMMODITY]: "Commodity",
  [SecurityType.CRYPTO]: "Crypto",
};

const REGIONS: Region[] = [Region.US, Region.INTERNATIONAL, Region.UK, Region.BONDS];

// Short forms for the assets table, where "Individual Stock or Bond" would
// blow out the column - the Select in the form keeps the full descriptive
// text from CATEGORY_LABELS.
const CATEGORY_BADGE_LABELS: Record<AssetCategory, string> = {
  [AssetCategory.UNSPECIFIED]: "Unknown",
  [AssetCategory.STOCK_OR_BOND]: "Stock/Bond",
  [AssetCategory.CASH]: "Cash",
  [AssetCategory.ETF]: "ETF",
  [AssetCategory.RETIREMENT]: "Retirement",
  [AssetCategory.REAL_ESTATE]: "Real Estate",
  [AssetCategory.LOAN]: "Loan",
  [AssetCategory.MORTGAGE]: "Mortgage",
  [AssetCategory.CAPITAL_BALANCE]: "Capital Balance",
};

const LIABILITY_CATEGORIES: AssetCategory[] = [AssetCategory.LOAN, AssetCategory.MORTGAGE];

// The new-asset form's initial liquidity tier, per category - overridable
// per instance (e.g. a restricted stock grant, or a locked CD). Mortgage
// defaults illiquid alongside Real Estate so hiding an illiquid house also
// hides its mortgage - otherwise "liquid net worth" would subtract a
// mortgage payment without the house value it's attached to. An ETF is
// liquid only when it's a bond fund (a money-market/T-bill fund like
// VBIL/SPAXX is a cash equivalent) - every other ETF, and every individual
// stock, counts as invested: accessible, but not without tax implications.
function defaultLiquidity(category: AssetCategory, securityType: SecurityType): AssetLiquidity {
  if (category === AssetCategory.CASH) return AssetLiquidity.LIQUID;
  if (category === AssetCategory.ETF && securityType === SecurityType.BOND) return AssetLiquidity.LIQUID;
  if (
    category === AssetCategory.RETIREMENT ||
    category === AssetCategory.REAL_ESTATE ||
    category === AssetCategory.MORTGAGE
  ) {
    return AssetLiquidity.ILLIQUID;
  }
  return AssetLiquidity.INVESTED;
}

const LIQUIDITY_LABELS: Record<AssetLiquidity, string> = {
  [AssetLiquidity.UNSPECIFIED]: "Unknown",
  [AssetLiquidity.LIQUID]: "Liquid",
  [AssetLiquidity.INVESTED]: "Invested",
  [AssetLiquidity.ILLIQUID]: "Illiquid",
};

// Shown as a caption under the form's Liquidity select, so the (somewhat
// judgment-call) boundaries are explicit rather than left to guess from the
// label alone - matches the same wording as the navbar's LiquidityFilter.
const LIQUIDITY_DESCRIPTIONS: Record<AssetLiquidity, string> = {
  [AssetLiquidity.UNSPECIFIED]: "",
  [AssetLiquidity.LIQUID]: "Cash & cash-equivalents - SPAXX, bond ETFs like VBIL",
  [AssetLiquidity.INVESTED]: "Accessible, but with tax implications - stocks, ETFs",
  [AssetLiquidity.ILLIQUID]: "Locked up - real estate, retirement accounts",
};

function categoryBadgeLabel(asset: Asset): string {
  if (asset.details.case === "etf") {
    return `ETF · ${SECURITY_TYPE_LABELS[asset.details.value.securityType]}`;
  }
  return CATEGORY_BADGE_LABELS[asset.category];
}

// The muted line under an asset's name - wherever it's held/owed, since
// that's the detail people actually scan for once the name is legible.
function assetDetailSubtitle(asset: Asset): string {
  switch (asset.details.case) {
    case "stockOrBond":
      return asset.details.value.platform;
    case "cash":
      return asset.details.value.institution;
    case "etf":
      return asset.details.value.platform;
    case "retirement":
      return asset.details.value.platform;
    case "realEstate":
      return "";
    case "loan":
      return asset.details.value.lender || "Informal";
    case "mortgage":
      return asset.details.value.institution;
    case "capitalBalance":
      return "";
    default:
      return "";
  }
}

function currencyCode(currency: Currency): string {
  return currency === Currency.GBP ? "GBP" : "USD";
}

function formatMoney(amountMinor: bigint, currency: Currency): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: currencyCode(currency),
  }).format(fromMinorUnits(amountMinor));
}

// Formats amountMinor in displayCurrency when a rate for date is available,
// falling back to the value's own native currency otherwise - a stale or
// not-yet-fetched rate should never block showing a number, just show the
// one we actually have.
function formatMoneyDisplay(
  amountMinor: bigint,
  nativeCurrency: Currency,
  date: string,
  displayCurrency: Currency,
  ratesByDate: Record<string, number>,
): string {
  const converted = convertAmount(amountMinor, nativeCurrency, displayCurrency, date, ratesByDate);
  if (converted === null) {
    return formatMoney(amountMinor, nativeCurrency);
  }
  return formatMoney(converted, displayCurrency);
}

// First name only, to keep the table's Owners column from ballooning -
// the form's ownership section still shows full names.
function ownersSummaryShort(asset: Asset): string {
  return asset.owners
    .map((owner) => `${owner.userName.split(" ")[0]} ${owner.percentage}%`)
    .join(", ");
}

function todayDate(): string {
  return new Date().toISOString().slice(0, 10);
}

function exposureSelectionsFrom(asset: Asset | null): Record<Region, string> {
  const selections: Record<Region, string> = {
    [Region.UNSPECIFIED]: "",
    [Region.US]: "",
    [Region.INTERNATIONAL]: "",
    [Region.UK]: "",
    [Region.BONDS]: "",
  };
  for (const region of asset?.exposures ?? []) {
    selections[region.region] = String(region.percentage);
  }
  return selections;
}

function sumPercentages(values: Record<string, string>): number {
  return Object.values(values).reduce((sum, value) => sum + (parseInt(value, 10) || 0), 0);
}

const columnHelper = createColumnHelper<Asset>();

function columnAlign(columnDef: { meta?: unknown }): "left" | "right" {
  const meta = columnDef.meta as { align?: "left" | "right" } | undefined;
  return meta?.align ?? "left";
}

interface AssetGroup {
  key: string;
  label: string;
  assets: Asset[];
  showOwners: boolean;
}

// Individual assets (a single 100%-owner) group under that owner's name;
// anything with more than one owner (regardless of split) groups under
// "Shared" - the Owners column is then only needed in the Shared group,
// since every other group's ownership is already implied by its heading.
// Owner labels/order come from the assets' own denormalized owners[].userName
// rather than requiring useUsers() to have loaded, matching the pattern
// already used by ownersSummaryShort - and preferred-order comes from the
// users list only when it's available, falling back to first-appearance
// order otherwise so nothing silently disappears while users is loading.
function groupAssetsByOwner(assets: Asset[], users: { id: string; name: string }[]): AssetGroup[] {
  const byOwner = new Map<string, { label: string; assets: Asset[] }>();
  const shared: Asset[] = [];
  for (const asset of assets) {
    if (asset.owners.length === 1) {
      const owner = asset.owners[0];
      const entry = byOwner.get(owner.userId) ?? { label: owner.userName, assets: [] };
      entry.assets.push(asset);
      byOwner.set(owner.userId, entry);
    } else {
      shared.push(asset);
    }
  }

  const groups: AssetGroup[] = [];
  const seen = new Set<string>();
  for (const user of users) {
    const entry = byOwner.get(user.id);
    if (entry) {
      groups.push({ key: user.id, label: entry.label, assets: entry.assets, showOwners: false });
      seen.add(user.id);
    }
  }
  for (const [ownerId, entry] of byOwner) {
    if (!seen.has(ownerId)) {
      groups.push({ key: ownerId, label: entry.label, assets: entry.assets, showOwners: false });
    }
  }
  if (shared.length > 0) {
    groups.push({ key: "shared", label: "Shared", assets: shared, showOwners: true });
  }
  return groups;
}

export function AssetsPage() {
  const { data: assets, isLoading, isError, error } = useAssets();
  const { data: users } = useUsers();
  const deleteAsset = useDeleteAsset();
  const { displayCurrency } = useDisplayCurrency();
  const [liquidityFilter] = useLiquidityFilter();

  const [formOpen, setFormOpen] = useState(false);
  const [editingAsset, setEditingAsset] = useState<Asset | null>(null);
  const [historyAsset, setHistoryAsset] = useState<Asset | null>(null);

  const valuationDates = (assets ?? [])
    .map((asset) => asset.latestValuation?.date)
    .filter((date): date is string => !!date);
  const { data: ratesByDate } = useExchangeRates(valuationDates);

  const filteredAssets = useMemo(
    () => (assets ?? []).filter((asset) => liquidityMatches(asset.liquidity, liquidityFilter)),
    [assets, liquidityFilter],
  );
  const groups = useMemo(() => groupAssetsByOwner(filteredAssets, users ?? []), [filteredAssets, users]);

  function handleEdit(asset: Asset) {
    setEditingAsset(asset);
    setFormOpen(true);
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="font-heading text-base font-medium">Assets</h1>
        <Button
          size="sm"
          className="gap-1.5"
          onClick={() => {
            setEditingAsset(null);
            setFormOpen(true);
          }}
        >
          <Plus />
          Add asset
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading assets…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {groups.map((group) => (
        <Card key={group.key} size="sm">
          <CardHeader>
            <CardTitle>{group.label}</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            <AssetsTable
              assets={group.assets}
              showOwners={group.showOwners}
              displayCurrency={displayCurrency}
              ratesByDate={ratesByDate ?? {}}
              deletePending={deleteAsset.isPending}
              onShowHistory={setHistoryAsset}
              onEdit={handleEdit}
              onDelete={(id) => deleteAsset.mutate(id)}
            />
          </CardContent>
        </Card>
      ))}
      {assets && assets.length === 0 && (
        <p className="text-sm text-muted-foreground">No assets yet.</p>
      )}
      {assets && assets.length > 0 && filteredAssets.length === 0 && (
        <p className="text-sm text-muted-foreground">No {liquidityFilter} assets.</p>
      )}
      {deleteAsset.isError && <p className="text-sm text-destructive">{deleteAsset.error.message}</p>}

      <Link to="/" className="text-sm text-muted-foreground underline underline-offset-4">
        Back
      </Link>

      <AssetFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        asset={editingAsset}
        users={users ?? []}
      />
      <ValuationHistoryDialog
        asset={historyAsset}
        onOpenChange={(open) => {
          if (!open) setHistoryAsset(null);
        }}
      />
    </div>
  );
}

interface AssetsTableProps {
  assets: Asset[];
  showOwners: boolean;
  displayCurrency: Currency;
  ratesByDate: Record<string, number>;
  deletePending: boolean;
  onShowHistory: (asset: Asset) => void;
  onEdit: (asset: Asset) => void;
  onDelete: (id: string) => void;
}

function AssetsTable({
  assets,
  showOwners,
  displayCurrency,
  ratesByDate,
  deletePending,
  onShowHistory,
  onEdit,
  onDelete,
}: AssetsTableProps) {
  const columns = useMemo(
    () => [
      columnHelper.display({
        id: "category",
        header: "Category",
        cell: (info) => (
          <div className="flex flex-wrap gap-1">
            <Badge variant={LIABILITY_CATEGORIES.includes(info.row.original.category) ? "destructive" : "secondary"}>
              {categoryBadgeLabel(info.row.original)}
            </Badge>
            {info.row.original.liquidity === AssetLiquidity.ILLIQUID && (
              <Badge variant="outline">Illiquid</Badge>
            )}
          </div>
        ),
      }),
      columnHelper.display({
        id: "name",
        header: "Name",
        cell: (info) => (
          <div className="grid gap-0.5">
            <span className="font-medium text-foreground">{assetDisplayName(info.row.original)}</span>
            {assetDetailSubtitle(info.row.original) && (
              <span className="text-xs text-muted-foreground">{assetDetailSubtitle(info.row.original)}</span>
            )}
          </div>
        ),
      }),
      columnHelper.display({
        id: "value",
        header: "Value",
        meta: { align: "right" as const },
        cell: (info) => {
          const valuation = info.row.original.latestValuation;
          if (!valuation) return "—";
          return formatMoneyDisplay(
            valuation.amount,
            info.row.original.currency,
            valuation.date,
            displayCurrency,
            ratesByDate,
          );
        },
      }),
      columnHelper.display({
        id: "lastUpdated",
        header: "Last Updated",
        cell: (info) => {
          const date = info.row.original.latestValuation?.date;
          return date ? formatDate(date) : "—";
        },
      }),
      ...(showOwners
        ? [
            columnHelper.display({
              id: "owners",
              header: "Owners",
              cell: (info) => (
                <span className="text-muted-foreground">{ownersSummaryShort(info.row.original)}</span>
              ),
            }),
          ]
        : []),
      columnHelper.display({
        id: "actions",
        meta: { align: "right" as const },
        cell: (info) => (
          <div className="flex justify-end gap-0.5">
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Valuation history for ${assetDisplayName(info.row.original)}`}
              onClick={() => onShowHistory(info.row.original)}
            >
              <History />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Edit ${assetDisplayName(info.row.original)}`}
              onClick={() => onEdit(info.row.original)}
            >
              <Pencil />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Delete ${assetDisplayName(info.row.original)}`}
              disabled={deletePending}
              onClick={() => onDelete(info.row.original.id)}
            >
              <Trash2 />
            </Button>
          </div>
        ),
      }),
    ],
    [showOwners, displayCurrency, ratesByDate, deletePending, onShowHistory, onEdit, onDelete],
  );

  const table = useReactTable({
    data: assets,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id} className="hover:bg-transparent">
            {headerGroup.headers.map((header) => (
              <TableHead
                key={header.id}
                className={cn(
                  "h-8 px-4 text-xs font-medium tracking-wide text-muted-foreground uppercase",
                  columnAlign(header.column.columnDef) === "right" && "text-right",
                )}
              >
                {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <TableCell
                key={cell.id}
                className={cn(
                  "px-4 py-3 tabular-nums",
                  columnAlign(cell.column.columnDef) === "right" && "text-right",
                )}
              >
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

interface AssetFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  asset: Asset | null;
  users: { id: string; name: string }[];
}

function AssetFormDialog({ open, onOpenChange, asset, users }: AssetFormDialogProps) {
  const isEditing = asset !== null;
  const createAsset = useCreateAsset();
  const updateAsset = useUpdateAsset();
  const mutation = isEditing ? updateAsset : createAsset;

  const [category, setCategory] = useState<AssetCategory>(asset?.category ?? AssetCategory.CASH);
  const [currency, setCurrency] = useState<Currency>(asset?.currency ?? Currency.USD);
  const [companyOrBondName, setCompanyOrBondName] = useState(
    asset?.details.case === "stockOrBond" ? asset.details.value.companyOrBondName : "",
  );
  const [platform, setPlatform] = useState(
    asset?.details.case === "stockOrBond"
      ? asset.details.value.platform
      : asset?.details.case === "etf"
        ? asset.details.value.platform
        : asset?.details.case === "retirement"
          ? asset.details.value.platform
          : "",
  );
  const [accountName, setAccountName] = useState(
    asset?.details.case === "cash" ? asset.details.value.accountName : "",
  );
  const [institution, setInstitution] = useState(
    asset?.details.case === "cash"
      ? asset.details.value.institution
      : asset?.details.case === "mortgage"
        ? asset.details.value.institution
        : "",
  );
  const [etfName, setEtfName] = useState(asset?.details.case === "etf" ? asset.details.value.etfName : "");
  const [securityType, setSecurityType] = useState<SecurityType>(
    asset?.details.case === "etf" ? asset.details.value.securityType : SecurityType.STOCK,
  );
  const [planName, setPlanName] = useState(
    asset?.details.case === "retirement" ? asset.details.value.planName : "",
  );
  const [propertyName, setPropertyName] = useState(
    asset?.details.case === "realEstate"
      ? asset.details.value.propertyName
      : asset?.details.case === "mortgage"
        ? asset.details.value.propertyName
        : "",
  );
  const [notes, setNotes] = useState(asset?.details.case === "realEstate" ? asset.details.value.notes : "");
  const [loanName, setLoanName] = useState(asset?.details.case === "loan" ? asset.details.value.name : "");
  const [lender, setLender] = useState(asset?.details.case === "loan" ? asset.details.value.lender : "");
  const [capitalBalanceName, setCapitalBalanceName] = useState(
    asset?.details.case === "capitalBalance" ? asset.details.value.name : "",
  );
  const [ownerSelections, setOwnerSelections] = useState<Record<string, string>>(() =>
    Object.fromEntries((asset?.owners ?? []).map((owner) => [owner.userId, String(owner.percentage)])),
  );
  const [exposureSelections, setExposureSelections] = useState<Record<Region, string>>(() =>
    exposureSelectionsFrom(asset),
  );
  const [initialDate, setInitialDate] = useState(todayDate());
  const [initialAmount, setInitialAmount] = useState("");
  const [liquidity, setLiquidity] = useState(asset?.liquidity ?? defaultLiquidity(category, securityType));

  function resetTo(next: Asset | null) {
    const nextCategory = next?.category ?? AssetCategory.CASH;
    const nextSecurityType = next?.details.case === "etf" ? next.details.value.securityType : SecurityType.STOCK;
    setCategory(nextCategory);
    setCurrency(next?.currency ?? Currency.USD);
    setLiquidity(next?.liquidity ?? defaultLiquidity(nextCategory, nextSecurityType));
    setCompanyOrBondName(next?.details.case === "stockOrBond" ? next.details.value.companyOrBondName : "");
    setPlatform(
      next?.details.case === "stockOrBond"
        ? next.details.value.platform
        : next?.details.case === "etf"
          ? next.details.value.platform
          : next?.details.case === "retirement"
            ? next.details.value.platform
            : "",
    );
    setAccountName(next?.details.case === "cash" ? next.details.value.accountName : "");
    setInstitution(
      next?.details.case === "cash"
        ? next.details.value.institution
        : next?.details.case === "mortgage"
          ? next.details.value.institution
          : "",
    );
    setEtfName(next?.details.case === "etf" ? next.details.value.etfName : "");
    setSecurityType(next?.details.case === "etf" ? next.details.value.securityType : SecurityType.STOCK);
    setPlanName(next?.details.case === "retirement" ? next.details.value.planName : "");
    setPropertyName(
      next?.details.case === "realEstate"
        ? next.details.value.propertyName
        : next?.details.case === "mortgage"
          ? next.details.value.propertyName
          : "",
    );
    setNotes(next?.details.case === "realEstate" ? next.details.value.notes : "");
    setLoanName(next?.details.case === "loan" ? next.details.value.name : "");
    setLender(next?.details.case === "loan" ? next.details.value.lender : "");
    setCapitalBalanceName(next?.details.case === "capitalBalance" ? next.details.value.name : "");
    setOwnerSelections(Object.fromEntries((next?.owners ?? []).map((owner) => [owner.userId, String(owner.percentage)])));
    setExposureSelections(exposureSelectionsFrom(next));
    setInitialDate(todayDate());
    setInitialAmount("");
  }

  function handleOpenChange(next: boolean) {
    if (next) resetTo(asset);
    onOpenChange(next);
  }

  function toggleOwner(userId: string) {
    setOwnerSelections((prev) => {
      const next = { ...prev };
      if (userId in next) {
        delete next[userId];
      } else {
        next[userId] = Object.keys(prev).length === 0 ? "100" : "";
      }
      return next;
    });
  }

  const ownerSum = sumPercentages(ownerSelections);
  const ownersValid = Object.keys(ownerSelections).length > 0 && ownerSum === 100;
  const exposureSum = sumPercentages(exposureSelections);
  // Geography is optional (commodities, crypto, and liabilities have no
  // meaningful region breakdown) - a blank split is fine, but a partial one
  // that doesn't sum to 100 isn't, matching the backend's relaxation.
  const exposuresValid = exposureSum === 0 || exposureSum === 100;

  const detailsValid =
    category === AssetCategory.STOCK_OR_BOND
      ? companyOrBondName.trim() !== "" && platform.trim() !== ""
      : category === AssetCategory.CASH
        ? accountName.trim() !== "" && institution.trim() !== ""
        : category === AssetCategory.ETF
          ? etfName.trim() !== "" && platform.trim() !== ""
          : category === AssetCategory.RETIREMENT
            ? planName.trim() !== "" && platform.trim() !== ""
            : category === AssetCategory.REAL_ESTATE
              ? propertyName.trim() !== ""
              : category === AssetCategory.LOAN
                ? loanName.trim() !== "" // lender is optional
                : category === AssetCategory.MORTGAGE
                  ? propertyName.trim() !== "" && institution.trim() !== ""
                  : category === AssetCategory.CAPITAL_BALANCE
                    ? capitalBalanceName.trim() !== ""
                    : false;

  const initialValuationValid = isEditing || (initialDate.trim() !== "" && initialAmount.trim() !== "");

  const canSubmit = ownersValid && exposuresValid && detailsValid && initialValuationValid;

  function buildDetails() {
    switch (category) {
      case AssetCategory.STOCK_OR_BOND:
        return {
          case: "stockOrBond" as const,
          value: { companyOrBondName: companyOrBondName.trim(), platform: platform.trim() },
        };
      case AssetCategory.CASH:
        return {
          case: "cash" as const,
          value: { accountName: accountName.trim(), institution: institution.trim() },
        };
      case AssetCategory.ETF:
        return {
          case: "etf" as const,
          value: { etfName: etfName.trim(), securityType, platform: platform.trim() },
        };
      case AssetCategory.RETIREMENT:
        return {
          case: "retirement" as const,
          value: { planName: planName.trim(), platform: platform.trim() },
        };
      case AssetCategory.REAL_ESTATE:
        return {
          case: "realEstate" as const,
          value: { propertyName: propertyName.trim(), notes: notes.trim() },
        };
      case AssetCategory.LOAN:
        return {
          case: "loan" as const,
          value: { name: loanName.trim(), lender: lender.trim() },
        };
      case AssetCategory.MORTGAGE:
        return {
          case: "mortgage" as const,
          value: { propertyName: propertyName.trim(), institution: institution.trim() },
        };
      case AssetCategory.CAPITAL_BALANCE:
        return {
          case: "capitalBalance" as const,
          value: { name: capitalBalanceName.trim() },
        };
      default:
        return { case: undefined };
    }
  }

  function buildOwners() {
    return Object.entries(ownerSelections).map(([userId, percentage]) => ({
      userId,
      percentage: parseInt(percentage, 10) || 0,
    }));
  }

  function buildExposures() {
    return REGIONS.filter((region) => exposureSelections[region].trim() !== "").map((region) => ({
      region,
      percentage: parseInt(exposureSelections[region], 10) || 0,
    }));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;

    if (isEditing && asset) {
      updateAsset.mutate(
        {
          id: asset.id,
          category,
          currency,
          owners: buildOwners(),
          exposures: buildExposures(),
          liquidity,
          details: buildDetails(),
        },
        { onSuccess: () => onOpenChange(false) },
      );
    } else {
      createAsset.mutate(
        {
          category,
          currency,
          owners: buildOwners(),
          exposures: buildExposures(),
          initialValuationDate: initialDate.trim(),
          initialValuationAmount: toMinorUnits(parseFloat(initialAmount) || 0),
          liquidity,
          details: buildDetails(),
        },
        { onSuccess: () => onOpenChange(false) },
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEditing ? "Edit asset" : "Add asset"}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="grid gap-4">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label>Category</Label>
              <Select
                value={String(category)}
                onValueChange={(v) => {
                  const next = Number(v) as AssetCategory;
                  setCategory(next);
                  setLiquidity(defaultLiquidity(next, securityType));
                }}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(AssetCategory.STOCK_OR_BOND)}>
                    {CATEGORY_LABELS[AssetCategory.STOCK_OR_BOND]}
                  </SelectItem>
                  <SelectItem value={String(AssetCategory.CASH)}>{CATEGORY_LABELS[AssetCategory.CASH]}</SelectItem>
                  <SelectItem value={String(AssetCategory.ETF)}>{CATEGORY_LABELS[AssetCategory.ETF]}</SelectItem>
                  <SelectItem value={String(AssetCategory.RETIREMENT)}>
                    {CATEGORY_LABELS[AssetCategory.RETIREMENT]}
                  </SelectItem>
                  <SelectItem value={String(AssetCategory.REAL_ESTATE)}>
                    {CATEGORY_LABELS[AssetCategory.REAL_ESTATE]}
                  </SelectItem>
                  <SelectItem value={String(AssetCategory.LOAN)}>{CATEGORY_LABELS[AssetCategory.LOAN]}</SelectItem>
                  <SelectItem value={String(AssetCategory.MORTGAGE)}>
                    {CATEGORY_LABELS[AssetCategory.MORTGAGE]}
                  </SelectItem>
                  <SelectItem value={String(AssetCategory.CAPITAL_BALANCE)}>
                    {CATEGORY_LABELS[AssetCategory.CAPITAL_BALANCE]}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>Currency</Label>
              <Select value={String(currency)} onValueChange={(v) => setCurrency(Number(v) as Currency)}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(Currency.USD)}>USD</SelectItem>
                  <SelectItem value={String(Currency.GBP)}>GBP</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {category === AssetCategory.STOCK_OR_BOND && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="company-or-bond-name">Company or bond</Label>
                <Input
                  id="company-or-bond-name"
                  value={companyOrBondName}
                  onChange={(e) => setCompanyOrBondName(e.target.value)}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="platform-stock">Platform</Label>
                <Input id="platform-stock" value={platform} onChange={(e) => setPlatform(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.CASH && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="account-name">Account name</Label>
                <Input id="account-name" value={accountName} onChange={(e) => setAccountName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="institution">Institution</Label>
                <Input id="institution" value={institution} onChange={(e) => setInstitution(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.ETF && (
            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2 grid gap-1.5">
                <Label htmlFor="etf-name">ETF</Label>
                <Input id="etf-name" value={etfName} onChange={(e) => setEtfName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label>Type</Label>
                <Select
                  value={String(securityType)}
                  onValueChange={(v) => {
                    const next = Number(v) as SecurityType;
                    setSecurityType(next);
                    setLiquidity(defaultLiquidity(category, next));
                  }}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={String(SecurityType.STOCK)}>Stock</SelectItem>
                    <SelectItem value={String(SecurityType.BOND)}>Bond</SelectItem>
                    <SelectItem value={String(SecurityType.COMMODITY)}>Commodity</SelectItem>
                    <SelectItem value={String(SecurityType.CRYPTO)}>Crypto</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="col-span-3 grid gap-1.5">
                <Label htmlFor="platform-etf">Platform</Label>
                <Input id="platform-etf" value={platform} onChange={(e) => setPlatform(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.RETIREMENT && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="plan-name">Plan name</Label>
                <Input id="plan-name" value={planName} onChange={(e) => setPlanName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="platform-retirement">Platform</Label>
                <Input id="platform-retirement" value={platform} onChange={(e) => setPlatform(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.REAL_ESTATE && (
            <div className="grid gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="property-name">Property name</Label>
                <Input id="property-name" value={propertyName} onChange={(e) => setPropertyName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="real-estate-notes">Notes</Label>
                <Input id="real-estate-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.LOAN && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="loan-name">Name</Label>
                <Input id="loan-name" value={loanName} onChange={(e) => setLoanName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="lender">Lender (optional)</Label>
                <Input id="lender" value={lender} onChange={(e) => setLender(e.target.value)} />
              </div>
            </div>
          )}

          {category === AssetCategory.MORTGAGE && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="mortgage-property-name">Property name</Label>
                <Input
                  id="mortgage-property-name"
                  value={propertyName}
                  onChange={(e) => setPropertyName(e.target.value)}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="mortgage-institution">Institution</Label>
                <Input
                  id="mortgage-institution"
                  value={institution}
                  onChange={(e) => setInstitution(e.target.value)}
                />
              </div>
            </div>
          )}

          {category === AssetCategory.CAPITAL_BALANCE && (
            <div className="grid gap-1.5">
              <Label htmlFor="capital-balance-name">Name</Label>
              <Input
                id="capital-balance-name"
                value={capitalBalanceName}
                onChange={(e) => setCapitalBalanceName(e.target.value)}
              />
            </div>
          )}

          <div className="grid gap-1.5">
            <div className="flex items-center justify-between">
              <Label>Ownership</Label>
              <span className={ownersValid ? "text-xs text-muted-foreground" : "text-xs text-destructive"}>
                {ownerSum}% / 100%
              </span>
            </div>
            <div className="grid gap-1.5 rounded-lg border p-2">
              {users.length === 0 && <p className="text-sm text-muted-foreground">No users yet.</p>}
              {users.map((user) => (
                <div key={user.id} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id={`owner-${user.id}`}
                    checked={user.id in ownerSelections}
                    onChange={() => toggleOwner(user.id)}
                    className="size-4"
                  />
                  <Label htmlFor={`owner-${user.id}`} className="flex-1 font-normal">
                    {user.name}
                  </Label>
                  {user.id in ownerSelections && (
                    <Input
                      type="number"
                      min={1}
                      max={100}
                      className="w-20"
                      value={ownerSelections[user.id]}
                      onChange={(e) =>
                        setOwnerSelections((prev) => ({ ...prev, [user.id]: e.target.value }))
                      }
                    />
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="grid gap-1.5">
            <div className="flex items-center justify-between">
              <Label>Exposure (optional)</Label>
              <span className={exposuresValid ? "text-xs text-muted-foreground" : "text-xs text-destructive"}>
                {exposureSum === 0 ? "none" : `${exposureSum}% / 100%`}
              </span>
            </div>
            <div className="grid gap-1.5 rounded-lg border p-2">
              {REGIONS.map((region) => (
                <div key={region} className="flex items-center gap-2">
                  <Label htmlFor={`region-${region}`} className="flex-1 font-normal">
                    {REGION_LABELS[region]}
                  </Label>
                  <Input
                    id={`region-${region}`}
                    type="number"
                    min={0}
                    max={100}
                    className="w-20"
                    value={exposureSelections[region]}
                    onChange={(e) =>
                      setExposureSelections((prev) => ({ ...prev, [region]: e.target.value }))
                    }
                  />
                </div>
              ))}
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label>Liquidity</Label>
            <Select value={String(liquidity)} onValueChange={(v) => setLiquidity(Number(v) as AssetLiquidity)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={String(AssetLiquidity.LIQUID)}>{LIQUIDITY_LABELS[AssetLiquidity.LIQUID]}</SelectItem>
                <SelectItem value={String(AssetLiquidity.INVESTED)}>
                  {LIQUIDITY_LABELS[AssetLiquidity.INVESTED]}
                </SelectItem>
                <SelectItem value={String(AssetLiquidity.ILLIQUID)}>
                  {LIQUIDITY_LABELS[AssetLiquidity.ILLIQUID]}
                </SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{LIQUIDITY_DESCRIPTIONS[liquidity]}</p>
          </div>

          {!isEditing && (
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="initial-date">Value as of</Label>
                <Input
                  id="initial-date"
                  type="date"
                  value={initialDate}
                  onChange={(e) => setInitialDate(e.target.value)}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="initial-amount">Value</Label>
                <Input
                  id="initial-amount"
                  type="number"
                  step="0.01"
                  min={category === AssetCategory.CAPITAL_BALANCE ? undefined : 0}
                  value={initialAmount}
                  onChange={(e) => setInitialAmount(e.target.value)}
                />
              </div>
            </div>
          )}

          {mutation.isError && <p className="text-sm text-destructive">{mutation.error.message}</p>}

          <DialogFooter>
            <Button type="submit" disabled={!canSubmit || mutation.isPending}>
              {isEditing ? "Save" : "Add"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

interface ValuationHistoryDialogProps {
  asset: Asset | null;
  onOpenChange: (open: boolean) => void;
}

function ValuationHistoryDialog({ asset, onOpenChange }: ValuationHistoryDialogProps) {
  const { data: valuations, isLoading } = useAssetValuations(asset?.id ?? "");
  const addValuation = useAddAssetValuation();
  const deleteValuation = useDeleteAssetValuation();
  const { displayCurrency } = useDisplayCurrency();
  const { data: ratesByDate } = useExchangeRates((valuations ?? []).map((v) => v.date));

  const [date, setDate] = useState(todayDate());
  const [amount, setAmount] = useState("");

  function handleAdd(e: FormEvent) {
    e.preventDefault();
    if (!asset || date.trim() === "" || amount.trim() === "") return;

    addValuation.mutate(
      { assetId: asset.id, date: date.trim(), amount: toMinorUnits(parseFloat(amount) || 0) },
      { onSuccess: () => setAmount("") },
    );
  }

  return (
    <Dialog open={asset !== null} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{asset ? `Valuation history — ${assetDisplayName(asset)}` : "Valuation history"}</DialogTitle>
        </DialogHeader>

        {asset && (
          <div className="grid gap-4">
            {isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
            {valuations && valuations.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {valuations.map((valuation: AssetValuation) => (
                    <TableRow key={valuation.id}>
                      <TableCell>{valuation.date}</TableCell>
                      <TableCell>
                        {formatMoneyDisplay(
                          valuation.amount,
                          asset.currency,
                          valuation.date,
                          displayCurrency,
                          ratesByDate ?? {},
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Delete valuation from ${valuation.date}`}
                          disabled={deleteValuation.isPending}
                          onClick={() =>
                            deleteValuation.mutate({ valuationId: valuation.id, assetId: asset.id })
                          }
                        >
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            {deleteValuation.isError && (
              <p className="text-sm text-destructive">{deleteValuation.error.message}</p>
            )}

            <form onSubmit={handleAdd} className="grid gap-1.5">
              <Label>Add valuation</Label>
              <div className="flex gap-2">
                <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
                <Input
                  type="number"
                  step="0.01"
                  min={asset.category === AssetCategory.CAPITAL_BALANCE ? undefined : 0}
                  placeholder="Amount"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
                <Button type="submit" disabled={addValuation.isPending}>
                  Add
                </Button>
              </div>
              {addValuation.isError && (
                <p className="text-sm text-destructive">{addValuation.error.message}</p>
              )}
            </form>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
