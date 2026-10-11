"""Reject downgrades and skipped Kubernetes minors before draining a host."""
import re
import sys


def version(text):
    match = re.search(r"v(\d+)\.(\d+)\.(\d+)\+k3s(\d+)", text)
    if not match:
        raise ValueError("Expected an exact stable K3s version: vMAJOR.MINOR.PATCH+k3sN")
    return tuple(map(int, match.groups()))


def validate(current, desired):
    old, new = version(current), version(desired)
    if new < old:
        raise ValueError("Downgrades require a separate reviewed recovery procedure")
    if new[0] != old[0] or new[1] > old[1] + 1:
        raise ValueError("Upgrade at most one Kubernetes minor version at a time")


if __name__ == "__main__":
    try:
        validate(*sys.argv[1:])
    except (ValueError, TypeError) as exc:
        sys.exit(str(exc))
