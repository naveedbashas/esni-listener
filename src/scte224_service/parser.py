"""XML parser that converts SCTE-224 media schedules into domain models."""

from __future__ import annotations

from pathlib import Path
import xml.etree.ElementTree as ET

from .models import (
    ApplyAction,
    MatchSignal,
    Media,
    MediaContent,
    MediaPoint,
    Policy,
    RemoveAction,
    SignalAssertion,
    ViewingPolicy,
)
from .utils import parse_bool, parse_datetime, parse_duration


NS = {
    "scte": "http://www.scte.org/schemas/224",
    "action": "http://qplive.com/schemas/224/action",
    "xlink": "http://www.w3.org/1999/xlink",
}


class MediaParser:
    """Parses SCTE-224 XML or JSON payloads into Python domain objects."""

    def parse_file(self, path: str | Path) -> Media:
        xml_text = Path(path).read_text(encoding="utf-8")
        return self.parse_xml(xml_text)

    def parse_xml(self, xml_text: str) -> Media:
        root = ET.fromstring(xml_text)
        if root.tag.split("}")[-1] != "Media":  # pragma: no cover - defensive
            raise ValueError("Root element must be <Media>")

        media = Media(
            id=root.get("id"),
            description=root.get("description"),
            last_updated=parse_datetime(root.get("lastUpdated")),
            effective=parse_datetime(root.get("effective")),
            expires=parse_datetime(root.get("expires")),
        )

        for mp_elem in root.findall("scte:MediaPoint", NS):
            media.media_points.append(self._parse_media_point(mp_elem))

        return media

    def _parse_media_point(self, element: ET.Element) -> MediaPoint:
        media_point = MediaPoint(
            id=element.get("id"),
            description=element.get("description"),
            effective=parse_datetime(element.get("effective")),
            expires=parse_datetime(element.get("expires")),
            match_time=parse_datetime(element.get("matchTime")),
            expected_duration=parse_duration(element.get("expectedDuration")),
            reusable=parse_bool(element.get("reusable")) if element.get("reusable") else False,
        )

        for alt in element.findall("scte:AltID", NS):
            alt_type = alt.get("type", "default")
            media_point.alt_ids[alt_type] = (alt.text or "").strip()

        for match_signal in element.findall("scte:MatchSignal", NS):
            media_point.match_signals.append(self._parse_match_signal(match_signal))

        for apply in element.findall("scte:Apply", NS):
            media_point.apply_actions.append(self._parse_apply(apply))

        for remove in element.findall("scte:Remove", NS):
            media_point.remove_actions.append(self._parse_remove(remove))

        return media_point

    def _parse_match_signal(self, element: ET.Element) -> MatchSignal:
        match_signal = MatchSignal(
            match=element.get("match", "ALL").upper(),
            schema=element.get("schema", ""),
        )

        for assertion in element.findall("scte:Assert", NS):
            text = (assertion.text or "").strip()
            if not text:
                continue
            match_signal.assertions.append(self._parse_assertion(text))

        return match_signal

    def _parse_assertion(self, expression: str) -> SignalAssertion:
        # Very small subset of XPath used in the sample schedule.
        expression = expression.strip()
        if expression.startswith("/"):
            expression = expression[1:]

        if "contains" in expression:
            path_part, contains_clause = expression.split("contains", 1)
            path = [p for p in path_part.split("/") if p]
            contains_clause = contains_clause.strip("()")
            attr_name, value = contains_clause.split(",")
            attr_name = attr_name.strip().lstrip("@")
            value = value.strip().strip("'\"")
            return SignalAssertion(path=path, contains={attr_name: value})

        if "[" not in expression:
            path = [p for p in expression.split("/") if p]
            return SignalAssertion(path=path)

        path_part, predicate_part = expression.split("[", 1)
        path = [p for p in path_part.split("/") if p]
        predicate_part = predicate_part.rstrip("]")

        equals = {}
        for clause in predicate_part.split(" and "):
            clause = clause.strip()
            if not clause:
                continue
            if "=" not in clause:
                continue
            name, value = clause.split("=", 1)
            name = name.strip().lstrip("@")
            value = value.strip().strip("'\"")
            equals[name] = value

        return SignalAssertion(path=path, equals=equals)

    def _parse_apply(self, element: ET.Element) -> ApplyAction:
        priority = int(element.get("priority", "0"))
        duration = parse_duration(element.get("duration"))
        policy_elem = element.find("scte:Policy", NS)
        if policy_elem is None:
            raise ValueError("<Apply> is missing nested <Policy>")

        policy = self._parse_policy(policy_elem)
        return ApplyAction(priority=priority, duration=duration, policy=policy)

    def _parse_remove(self, element: ET.Element) -> RemoveAction:
        policy_elem = element.find("scte:Policy", NS)
        if policy_elem is None:
            raise ValueError("<Remove> is missing nested <Policy>")
        href = policy_elem.get(f"{{{NS['xlink']}}}href")
        if not href:
            raise ValueError("<Remove><Policy> must provide xlink:href")
        return RemoveAction(policy_href=href)

    def _parse_policy(self, element: ET.Element) -> Policy:
        viewing_policies = []
        for vp_elem in element.findall("scte:ViewingPolicy", NS):
            viewing_policies.append(self._parse_viewing_policy(vp_elem))

        return Policy(
            id=element.get("id", ""),
            viewing_policies=viewing_policies,
        )

    def _parse_viewing_policy(self, element: ET.Element) -> ViewingPolicy:
        audience_elem = element.find("scte:Audience", NS)
        audience_id = audience_elem.get("id") if audience_elem is not None else None

        content_elem = element.find("action:Content", NS)
        if content_elem is None:
            content = MediaContent()
        else:
            content = MediaContent(
                source_uri=self._find_text(content_elem, "action:SourceURI"),
                alternate_uri=self._find_text(content_elem, "action:AlternateURI"),
                fallback_uri=self._find_text(content_elem, "action:FallbackURI"),
            )

        return ViewingPolicy(
            id=element.get("id", ""),
            audience_id=audience_id,
            content=content,
        )

    @staticmethod
    def _find_text(parent: ET.Element, path: str) -> str | None:
        node = parent.find(path, NS)
        if node is None or node.text is None:
            return None
        return node.text.strip()
