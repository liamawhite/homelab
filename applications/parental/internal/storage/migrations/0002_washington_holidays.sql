-- Washington State public holidays, 2027–2028, using observed weekdays.
-- Sources (verified 2026-09-18):
-- https://dor.wa.gov/contact/state-holiday-schedule (2027 schedule)
-- https://app.leg.wa.gov/rcw/default.aspx?cite=1.16.050 (holiday and observance rules)
-- 2028 dates are calculated from these rules; shared dates are cross-checked
-- against https://www.opm.gov/policy-data-oversight/pay-leave/federal-holidays/.
-- New Year's Day 2028 is observed on Dec 31, 2027 (a cross-year holiday).
-- Columbus Day is not a Washington State legal holiday.
INSERT INTO holidays(date,name) VALUES
('2027-01-01','New Year''s Day'),
('2027-01-18','Martin Luther King Jr. Day'),
('2027-02-15','Presidents'' Day'),
('2027-05-31','Memorial Day'),
('2027-06-18','Juneteenth (observed)'),
('2027-07-05','Independence Day (observed)'),
('2027-09-06','Labor Day'),
('2027-11-11','Veterans Day'),
('2027-11-25','Thanksgiving Day'),
('2027-11-26','Native American Heritage Day'),
('2027-12-24','Christmas Day (observed)'),
('2027-12-31','New Year''s Day 2028 (observed)'),
('2028-01-17','Martin Luther King Jr. Day'),
('2028-02-21','Presidents'' Day'),
('2028-05-29','Memorial Day'),
('2028-06-19','Juneteenth'),
('2028-07-04','Independence Day'),
('2028-09-04','Labor Day'),
('2028-11-10','Veterans Day (observed)'),
('2028-11-23','Thanksgiving Day'),
('2028-11-24','Native American Heritage Day'),
('2028-12-25','Christmas Day');
