"""Domain models for SCTE-224 media orchestration."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Dict, List, Optional


@dataclass(slots=True)
class SignalAssertion:
    """Represents a single assertion within a MatchSignal clause."""

    path: List[str]
    equals: Dict[str, str] = field(default_factory=dict)
    contains: Dict[str, str] = field(default_factory=dict)


@dataclass(slots=True)
class MatchSignal:
    match: str
    schema: str
    assertions: List[SignalAssertion] = field(default_factory=list)


@dataclass(slots=True)
class MediaContent:
    source_uri: Optional[str] = None
    alternate_uri: Optional[str] = None
    fallback_uri: Optional[str] = None


@dataclass(slots=True)
class ViewingPolicy:
    id: str
    audience_id: Optional[str]
    content: MediaContent


@dataclass(slots=True)
class Policy:
    id: str
    viewing_policies: List[ViewingPolicy]


@dataclass(slots=True)
class ApplyAction:
    priority: int
    policy: Policy
    duration: Optional[timedelta] = None


@dataclass(slots=True)
class RemoveAction:
    policy_href: str


@dataclass(slots=True)
class MediaPoint:
    id: str
    description: Optional[str]
    effective: Optional[datetime]
    expires: Optional[datetime]
    match_time: Optional[datetime]
    expected_duration: Optional[timedelta]
    reusable: bool
    alt_ids: Dict[str, str] = field(default_factory=dict)
    match_signals: List[MatchSignal] = field(default_factory=list)
    apply_actions: List[ApplyAction] = field(default_factory=list)
    remove_actions: List[RemoveAction] = field(default_factory=list)


@dataclass(slots=True)
class Media:
    id: str
    description: Optional[str]
    last_updated: Optional[datetime]
    effective: Optional[datetime]
    expires: Optional[datetime]
    media_points: List[MediaPoint] = field(default_factory=list)


@dataclass(slots=True)
class PolicyDecision:
    """Concrete playback instructions produced by the policy engine."""

    media_point_id: str
    policy_id: str
    viewing_policy_id: str
    audience_id: Optional[str]
    source_uri: Optional[str]
    alternate_uri: Optional[str]
    fallback_uri: Optional[str]
    priority: int
    triggered_at: datetime
    trigger_type: str
