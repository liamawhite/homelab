-- color is constrained to storage.LabelPalette - add new values there and
-- to this CHECK constraint together. Auto-assigned on create by rotating
-- through the palette (see storage.Labels.Create), editable afterward
-- (still only to another palette value) via SetColor/UpdateLabelColor.
ALTER TABLE labels ADD COLUMN color TEXT NOT NULL DEFAULT '#e03131' CHECK (color IN (
    '#e03131', '#f08c00', '#f5c518', '#2f9e44', '#0ca678',
    '#1971c2', '#4263eb', '#7048e8', '#ae3ec9', '#e64980'
));
