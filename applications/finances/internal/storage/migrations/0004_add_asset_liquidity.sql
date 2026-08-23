-- Plain additive column - unlike 0001/0003's CHECK-constraint changes, this
-- needs no rebuild-table technique (no FK-cascade risk). Every existing row
-- defaults to liquid, then gets backfilled per category: retirement/
-- real_estate/mortgage default illiquid (mortgage groups with real_estate so
-- hiding an illiquid house also hides its mortgage, keeping "liquid net
-- worth" coherent), everything else stays liquid. Individual assets can be
-- overridden from this default afterward via UpdateAsset.
ALTER TABLE assets ADD COLUMN is_liquid INTEGER NOT NULL DEFAULT 1;

UPDATE assets SET is_liquid = 0
    WHERE category IN ('retirement', 'real_estate', 'mortgage');
