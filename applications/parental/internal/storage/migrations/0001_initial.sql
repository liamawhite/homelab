CREATE TABLE plan (id INTEGER PRIMARY KEY CHECK(id = 1), birth_date TEXT NOT NULL, actual INTEGER NOT NULL DEFAULT 0);
CREATE TABLE people (id TEXT PRIMARY KEY, name TEXT NOT NULL, color TEXT NOT NULL, budget_days INTEGER CHECK(budget_days >= 0));
CREATE TABLE assignments (id TEXT PRIMARY KEY, person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE, start_date TEXT NOT NULL, end_date TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', CHECK(start_date <= end_date));
CREATE INDEX assignments_person ON assignments(person_id);
CREATE TABLE holidays (date TEXT PRIMARY KEY, name TEXT NOT NULL);
