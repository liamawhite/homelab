> **Disclaimer:** This repository is mostly vibe-coded, single-user software.

# Parental

Application source and multistage container build for `parental` in the homelab
monorepo. Run commands below from `applications/parental`.

## Development

The multistage Dockerfile installs the build tools and builds both the frontend
and backend. Build the application image with:

```sh
docker build -t parental .
```

For local development, use Go (version from `go.mod`) and Node.js 22.

```sh
make build
make test
make format-check
make lint
```

`make frontend` builds the React UI and copies it into the Go embed directory.
Generated Go and TypeScript bindings are committed; use `make generate` after
changing protobufs.

## Container publishing

Changes to `applications/parental/` on `main` publish `ghcr.io/liamawhite/parental:sha-<full-commit-sha>`
for `linux/amd64` and `linux/arm64`. The workflow updates `:latest` only while the
built commit remains the head of `main`. Publishing uses the `GHCR_TOKEN` Actions secret for the existing package, falling
back to the repository's `GITHUB_TOKEN` when no registry secret is configured.
Pull requests run independent format, lint, and backend test jobs.
The workflows follow [melodical](https://github.com/liamawhite/melodical/tree/470ea3e5f55c5eb45b722d2d83bfa961bf8450c3/.github/workflows).

Deploy an immutable SHA tag or digest from homelab; publishing an image does not
apply changes to the cluster.

The existing package predates this repository and initially rejected its
`GITHUB_TOKEN` with `permission_denied: write_package`. The `GHCR_TOKEN` Actions
secret supplies the existing homelab publishing credential without storing it in
source. To switch to `GITHUB_TOKEN`, grant the homelab repository **Write** permission
under [Manage Actions access](https://github.com/users/liamawhite/packages/container/parental/settings)
and remove the `GHCR_TOKEN` secret.

## Planning rules

- The planning window starts three weeks before birth and includes the first birthday, marked “Day care”.
  Pre-birth assignments are optional and consume leave budget; required coverage starts at birth and ends the day before the first birthday.
  February 29 birthdays use February 28 the following year.
- Assignments use inclusive, fixed calendar dates. Changing the birth date never
  moves or deletes them. Dates outside the planning window remain in the assignment list and use
  budget, but do not contribute to the year's coverage.
- Monday–Friday requires coverage, excluding configured public holidays.
  Public holidays and weekends never consume leave budget.
- Each person's budget counts the union of their assigned working days. Multiple
  caregivers can cover the same date. A blank budget is unlimited; zero is valid.
- Vacations are named, inclusive date ranges displayed on the calendar, including weekends.
  They do not change coverage or budgets; assign people separately for leave accounting.
  Changing the birth date preserves vacation dates.
- Exceeding a budget is allowed and displayed as a warning.
- Deleting a person also deletes their assignments, after confirmation in the UI.

## Washington State holidays

The calendar includes the 2027 and 2028 Washington State public holidays, including
Native American Heritage Day and observed weekdays. New Year's Day 2028 is
observed on **December 31, 2027**, so that date is excluded too. Columbus Day is
not a Washington State legal holiday. Personal/floating holidays are not included.

The 2027 dates follow the [state holiday schedule](https://dor.wa.gov/contact/state-holiday-schedule).
The 2028 dates are calculated from [RCW 1.16.050](https://app.leg.wa.gov/rcw/default.aspx?cite=1.16.050),
including its Saturday/Friday and Sunday/Monday observance rules. Shared federal
dates were cross-checked against the [OPM calendar](https://www.opm.gov/policy-data-oversight/pay-leave/federal-holidays/).
Sources were checked September 18, 2026. The app reads the seeded calendar locally;
no external holiday service is needed at runtime. Extend the holiday migration
if using the planner beyond 2027–2028.
