"""Score unmodified Presidio Analyzer defaults on the synthetic corpus.

Requires presidio-analyzer 2.2.364 and its default en_core_web_lg spaCy model.
Presidio is run with language="en" because its default pipeline is English.
Only the entity types mapped below belong to this benchmark's scope.
"""

from __future__ import annotations

import json
from collections import defaultdict
from importlib.metadata import version
from pathlib import Path

from presidio_analyzer import AnalyzerEngine

CORPUS = Path("bench/corpus/v0.jsonl")
MAPPING = {
    "PERSON": "name_demo",
    "EMAIL_ADDRESS": "email",
    "IBAN_CODE": "iban",
    "CREDIT_CARD": "card",
    "PHONE_NUMBER": "phone",
}


def exact_span(text: str, value: str) -> tuple[int, int]:
    if text.count(value) != 1:
        raise ValueError("annotation must appear exactly once")
    start = text.index(value)
    return len(text[:start].encode("utf-8")), len(text[: start + len(value)].encode("utf-8"))


def score() -> dict:
    analyzer = AnalyzerEngine()
    totals: dict[str, dict[str, int]] = defaultdict(lambda: {"tp": 0, "fp": 0, "fn": 0})
    seen_ids: set[str] = set()
    samples = 0
    for line in CORPUS.read_text(encoding="utf-8").splitlines():
        samples += 1
        item = json.loads(line)
        if item["id"] in seen_ids:
            raise ValueError("duplicate sample id")
        seen_ids.add(item["id"])
        text = item["text"]
        gold = {(entity["kind"], *exact_span(text, entity["value"])) for entity in item["entities"]}
        for kind, _, _ in gold:
            totals[kind]
        for result in analyzer.analyze(text=text, language="en"):
            kind = MAPPING.get(result.entity_type)
            if kind is None:
                continue
            start = len(text[: result.start].encode("utf-8"))
            end = len(text[: result.end].encode("utf-8"))
            span = (kind, start, end)
            if span in gold:
                totals[kind]["tp"] += 1
                gold.remove(span)
            else:
                totals[kind]["fp"] += 1
        for kind, _, _ in gold:
            totals[kind]["fn"] += 1
    per_kind = {}
    for kind, counts in sorted(totals.items()):
        tp, fp, fn = counts["tp"], counts["fp"], counts["fn"]
        per_kind[kind] = {
            **counts,
            "precision": tp / (tp + fp) if tp + fp else 0,
            "recall": tp / (tp + fn) if tp + fn else 0,
        }
    return {
        "corpus": str(CORPUS),
        "detector": "presidio-default-en",
        "presidio_version": version("presidio-analyzer"),
        "spacy_model": "en_core_web_lg 3.8.0",
        "language": "en",
        "samples": samples,
        "scoring": "exact span and entity type",
        "per_kind": per_kind,
    }


if __name__ == "__main__":
    print(json.dumps(score(), indent=2, ensure_ascii=False))
