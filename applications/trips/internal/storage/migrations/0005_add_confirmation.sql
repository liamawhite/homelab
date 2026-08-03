ALTER TABLE flights ADD COLUMN confirmation_link TEXT NOT NULL DEFAULT '';
ALTER TABLE flights ADD COLUMN booking_number TEXT NOT NULL DEFAULT '';
ALTER TABLE accommodations ADD COLUMN confirmation_link TEXT NOT NULL DEFAULT '';
ALTER TABLE accommodations ADD COLUMN booking_number TEXT NOT NULL DEFAULT '';
