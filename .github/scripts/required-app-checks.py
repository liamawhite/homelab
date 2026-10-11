"""Require the checks for changed apps without requiring skipped workflows."""
import json
import os
import subprocess
import time

APPS = ("parental", "shopping", "lumenetes", "workouts", "dashboard", "melodical")
CHECKS = ("format", "lint", "test")


def changed_apps(paths):
    return [app for app in APPS if any(
        path.startswith(f"applications/{app}/")
        or path in (f".github/workflows/{app}-checks.yml", f".github/workflows/{app}-publish.yml")
        for path in paths
    )]


def api(endpoint):
    return json.loads(subprocess.check_output(["gh", "api", endpoint], text=True))


def main():
    with open(os.environ["GITHUB_EVENT_PATH"]) as event_file:
        pr = json.load(event_file)["pull_request"]
    paths = subprocess.check_output([
        "git", "diff", "--name-only", f'{pr["base"]["sha"]}...{pr["head"]["sha"]}'
    ], text=True).splitlines()
    apps = changed_apps(paths)
    print("Changed apps: " + (", ".join(apps) or "none"), flush=True)
    if not apps:
        return
    repo = os.environ["GITHUB_REPOSITORY"]
    sha = pr["head"]["sha"]
    deadline = time.monotonic() + 25 * 60
    while time.monotonic() < deadline:
        runs = api(f"repos/{repo}/actions/runs?event=pull_request&head_sha={sha}&per_page=100")["workflow_runs"]
        waiting = []
        for app in apps:
            matches = [run for run in runs if run["path"] == f".github/workflows/{app}-checks.yml"
                       and run.get("pull_requests") and any(p["number"] == pr["number"] for p in run["pull_requests"])]
            if not matches:
                waiting.append(app + ": workflow not started")
                continue
            run = max(matches, key=lambda item: item["id"])
            if run["status"] != "completed":
                waiting.append(app + ": checks running")
                continue
            if run["conclusion"] != "success":
                raise SystemExit(f'{app}: workflow {run["conclusion"]}: {run["html_url"]}')
            jobs = api(f'repos/{repo}/actions/runs/{run["id"]}/jobs?per_page=100')["jobs"]
            for check in CHECKS:
                found = [job for job in jobs if job["name"] == check]
                if not found or any(job["conclusion"] != "success" for job in found):
                    raise SystemExit(f"{app}: missing or unsuccessful {check} check")
        if not waiting:
            print("All changed applications passed format, lint, and test.", flush=True)
            return
        print("; ".join(waiting), flush=True)
        time.sleep(15)
    raise SystemExit("Timed out waiting for required application checks")


if __name__ == "__main__":
    main()
