"""Utility helpers for date/time and duration parsing."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import re
from typing import Optional


_ISO_DURATION_PATTERN = re.compile(
    r"^P(?:(?P<days>\d+)D)?(?:T(?:(?P<hours>\d+)H)?(?:(?P<minutes>\d+)M)?(?:(?P<seconds>\d+)S)?)?$"
)


def parse_datetime(value: Optional[str]) -> Optional[datetime]:
    """Parse an ISO-8601 datetime string with optional 'Z'."""

    if not value:
        return None
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"
    try:
        dt = datetime.fromisoformat(value)
    except ValueError as exc:  # pragma: no cover - defensive
        raise ValueError(f"Unsupported datetime format: {value}") from exc
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def parse_duration(value: Optional[str]) -> Optional[timedelta]:
    """Parse a subset of ISO-8601 durations (PnDTnHnMnS)."""

    if not value:
        return None
    match = _ISO_DURATION_PATTERN.match(value)
    if not match:
        raise ValueError(f"Unsupported ISO-8601 duration: {value}")
    parts = {k: int(v) if v else 0 for k, v in match.groupdict().items()}
    return timedelta(
        days=parts["days"],
        hours=parts["hours"],
        minutes=parts["minutes"],
        seconds=parts["seconds"],
    )


def parse_bool(value: Optional[str]) -> bool:
    """Parse SCTE boolean values (default to False)."""

    return str(value).lower() in {"1", "true", "yes"}
