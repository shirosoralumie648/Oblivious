#!/usr/bin/env python3
"""Validate Phase 31.1 security and Nyquist audit artifacts structurally."""

from __future__ import annotations

import argparse
import html
import re
import sys
from dataclasses import dataclass
from pathlib import Path


PLAN_NUMBERS = tuple(range(11, 23))
SECURITY_HEADER = (
    "Threat ID",
    "Severity",
    "Evidence phase",
    "Source boundary",
    "Evidence command",
    "Observed result",
    "Commit identity",
    "Status",
)
VALIDATION_HEADER = (
    "Task ID",
    "Plan",
    "Task ordinal",
    "Selector/assertion IDs",
    "Automated result",
    "Audited commit",
)
THREAT_HEADER = (
    "Threat ID",
    "Category",
    "Component",
    "Severity",
    "Disposition",
    "Mitigation Plan",
)
PLACEHOLDERS = {"", "-", "n/a", "na", "none", "null", "tbd", "todo", "pending", "unknown"}
LEGACY_VALIDATION_TOKENS = (
    "target_plans: 10",
    "28 executable tasks",
    "28/28",
    "target_plans: 20",
    "gap_plans: 10",
    "mapped_gap_tasks: 22",
    "mapped_gap_evidence_pairs: 35",
    "unique_gap_evidence_ids: 32",
    "wave_0_complete: false",
    "pending implementation",
)
EXPECTED_VALIDATION_META = {
    "target_plans": "22",
    "gap_plans": "12",
    "mapped_gap_tasks": "26",
    "mapped_gap_evidence_pairs": "43",
    "unique_gap_evidence_ids": "40",
    "wave_0_complete": "true",
}
EXPECTED_SECURITY_META = {
    "high_threat_rows": "42",
    "precloseout_high_threats_open": "0",
    "post_closeout_high_threats_pending": "6",
    "post_closeout_gate_status": "pending_external",
    "release_eligible": "false",
}


class AuditError(ValueError):
    pass


@dataclass(frozen=True)
class ExpectedTask:
    task_id: str
    plan: int
    ordinal: int
    evidence_ids: tuple[str, ...]


def fail(message: str) -> None:
    raise AuditError(message)


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        fail(f"cannot read {path}: {exc}")


def parse_frontmatter(text: str, label: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        fail(f"{label} must start with YAML frontmatter")
    try:
        end = next(index for index in range(1, len(lines)) if lines[index].strip() == "---")
    except StopIteration:
        fail(f"{label} frontmatter is not terminated")
    result: dict[str, str] = {}
    for line in lines[1:end]:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if line[:1].isspace() or ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key in result:
            fail(f"{label} duplicate frontmatter key: {key}")
        result[key] = value
    return result


def section_lines(text: str, heading: str, label: str) -> tuple[list[str], int, int]:
    lines = text.splitlines()
    matches = [index for index, line in enumerate(lines) if line.strip() == heading]
    if len(matches) != 1:
        fail(f"{label} must contain exactly one {heading} section")
    start = matches[0] + 1
    end = len(lines)
    for index in range(start, len(lines)):
        if lines[index].startswith("## "):
            end = index
            break
    return lines[start:end], start, end


def split_markdown_row(line: str) -> tuple[str, ...]:
    stripped = line.strip()
    if not stripped.startswith("|") or not stripped.endswith("|"):
        fail(f"malformed markdown table row: {line}")
    return tuple(html.unescape(cell.strip()) for cell in stripped[1:-1].split("|"))


def is_separator(row: tuple[str, ...]) -> bool:
    return bool(row) and all(re.fullmatch(r":?-{3,}:?", cell) for cell in row)


def parse_section_table(text: str, heading: str, header: tuple[str, ...], label: str) -> tuple[list[tuple[str, ...]], tuple[int, int]]:
    lines, section_start, section_end = section_lines(text, heading, label)
    candidates = [(index, line) for index, line in enumerate(lines) if line.strip().startswith("|")]
    if len(candidates) < 2:
        fail(f"{label} {heading} must contain a markdown table")
    header_positions = [index for index, line in candidates if split_markdown_row(line) == header]
    if len(header_positions) != 1:
        fail(f"{label} {heading} must use exact header: {' | '.join(header)}")
    header_index = header_positions[0]
    if header_index + 1 >= len(lines) or not is_separator(split_markdown_row(lines[header_index + 1])):
        fail(f"{label} {heading} table separator is missing or malformed")
    rows: list[tuple[str, ...]] = []
    index = header_index + 2
    while index < len(lines) and lines[index].strip().startswith("|"):
        row = split_markdown_row(lines[index])
        if len(row) != len(header):
            fail(f"{label} table row has {len(row)} cells, expected {len(header)}")
        rows.append(row)
        index += 1
    if not rows:
        fail(f"{label} {heading} table has no data rows")
    other_table_lines = [
        position for position, line in candidates
        if position < header_index or position >= index
    ]
    if other_table_lines:
        fail(f"{label} {heading} contains table rows outside the designated table")
    absolute_start = section_start + header_index
    absolute_end = section_start + index
    return rows, (absolute_start, absolute_end)


def parse_plan_threats(text: str, plan: int) -> list[str]:
    rows, _ = parse_section_table(text, "## STRIDE Threat Register", THREAT_HEADER, f"Plan {plan}")
    high: list[str] = []
    seen: set[str] = set()
    for row in rows:
        threat_id, _category, _component, severity, disposition, mitigation = row
        match = re.fullmatch(r"T-31\.1-(\d{2})-(\d{2})", threat_id)
        if match is None or int(match.group(1)) != plan:
            fail(f"Plan {plan} has malformed or foreign threat id: {threat_id}")
        if threat_id in seen:
            fail(f"Plan {plan} has duplicate threat id: {threat_id}")
        seen.add(threat_id)
        if severity not in {"low", "medium", "high", "critical"}:
            fail(f"Plan {plan} threat {threat_id} has invalid severity: {severity}")
        if disposition in PLACEHOLDERS or mitigation.lower() in PLACEHOLDERS:
            fail(f"Plan {plan} threat {threat_id} has incomplete disposition or mitigation")
        if severity == "high":
            high.append(threat_id)
    return high


def extract_single(block: str, tag: str, label: str, required: bool = True) -> str:
    matches = re.findall(rf"<{tag}>(.*?)</{tag}>", block, flags=re.DOTALL)
    if not matches:
        if required:
            fail(f"{label} is missing <{tag}>")
        return ""
    if len(matches) != 1:
        fail(f"{label} has duplicate <{tag}> fields")
    return html.unescape(matches[0].strip())


def parse_plan_tasks(text: str, plan: int) -> list[ExpectedTask]:
    blocks = re.findall(r"<task\b[^>]*>(.*?)</task>", text, flags=re.DOTALL)
    if not blocks:
        fail(f"Plan {plan} has no task blocks")
    tasks: list[ExpectedTask] = []
    for ordinal, block in enumerate(blocks, start=1):
        label = f"Plan {plan} task {ordinal}"
        extract_single(block, "name", label)
        automated = extract_single(extract_single(block, "verify", label), "automated", label)
        test_ids = re.findall(r"\bTest[A-Za-z0-9_]+\b", automated)
        assertions_text = extract_single(block, "assertion_ids", label, required=False)
        assertion_ids: list[str] = []
        if assertions_text:
            for token in assertions_text.split(","):
                token = token.strip()
                if not re.fullmatch(r"[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+", token):
                    fail(f"{label} has malformed assertion id: {token}")
                assertion_ids.append(token)
        evidence_ids = test_ids + assertion_ids
        if not evidence_ids:
            fail(f"{label} has no automated Test ID or assertion ID")
        if len(evidence_ids) != len(set(evidence_ids)):
            fail(f"{label} has duplicate evidence ids")
        tasks.append(ExpectedTask(f"31.1-{plan:02d}-{ordinal:02d}", plan, ordinal, tuple(evidence_ids)))
    return tasks


def derive_expected(phase_dir: Path) -> tuple[list[str], list[ExpectedTask]]:
    threats: list[str] = []
    tasks: list[ExpectedTask] = []
    for plan in PLAN_NUMBERS:
        path = phase_dir / f"31.1-{plan:02d}-PLAN.md"
        if not path.is_file():
            fail(f"required continuous plan is absent: {path.name}")
        text = read_text(path)
        threats.extend(parse_plan_threats(text, plan))
        tasks.extend(parse_plan_tasks(text, plan))
    if len(threats) != len(set(threats)):
        fail("duplicate expected high-threat id across Plans 11-22")
    if len(threats) != 42:
        fail(f"derived high-threat count must equal 42, got {len(threats)}")
    pairs = {(task.task_id, evidence_id) for task in tasks for evidence_id in task.evidence_ids}
    evidence = {evidence_id for task in tasks for evidence_id in task.evidence_ids}
    if len(tasks) != 26:
        fail(f"derived task count must equal 26, got {len(tasks)}")
    if len(pairs) != 43:
        fail(f"derived task/evidence pair count must equal 43, got {len(pairs)}")
    if len(evidence) != 40:
        fail(f"derived unique evidence id count must equal 40, got {len(evidence)}")
    return threats, tasks


def require_meta(meta: dict[str, str], expected: dict[str, str], label: str) -> None:
    for key, value in expected.items():
        if meta.get(key) != value:
            fail(f"{label} frontmatter {key} must equal {value}")


def is_placeholder(value: str) -> bool:
    lowered = value.strip().lower()
    return lowered in PLACEHOLDERS or "placeholder" in lowered


def verify_security(text: str, expected_threats: list[str], expected_head: str) -> None:
    meta = parse_frontmatter(text, "SECURITY")
    require_meta(meta, EXPECTED_SECURITY_META, "SECURITY")
    if meta.get("audited_implementation_head") != expected_head:
        fail("SECURITY audited_implementation_head must equal expected implementation head")
    rows, _ = parse_section_table(text, "## High-Threat Audit Rows", SECURITY_HEADER, "SECURITY")
    by_id: dict[str, tuple[str, ...]] = {}
    for row in rows:
        threat_id, severity, evidence_phase, boundary, command, result, identity, status = row
        if threat_id in by_id:
            fail(f"duplicate security threat id: {threat_id}")
        by_id[threat_id] = row
        for field_name, value in (
            ("source boundary", boundary),
            ("evidence command", command),
            ("observed result", result),
            ("commit identity", identity),
        ):
            if is_placeholder(value):
                fail(f"security {threat_id} {field_name} is blank or placeholder")
        if severity != "high":
            fail(f"security {threat_id} severity must equal high")
        if not re.match(r"^(?:bash|python3?|jq|cd)\s+", command):
            fail(f"security {threat_id} evidence command is not executable")
        if threat_id.startswith("T-31.1-20-"):
            if evidence_phase != "post_closeout_external":
                fail(f"post-closeout row has invalid evidence phase: {threat_id}")
            if result != "NOT_OBSERVED_PRE_CLOSEOUT":
                fail(f"post-closeout row has invalid observed result: {threat_id}")
            if identity != "PENDING_FINAL_TRACKING_HEAD":
                fail(f"post-closeout row has invalid commit identity: {threat_id}")
            if status != "pending_external_gate":
                fail(f"post-closeout row has invalid status: {threat_id}")
        else:
            if evidence_phase != "pre_closeout":
                fail(f"pre-closeout row has invalid evidence phase: {threat_id}")
            if result != "PASS":
                fail(f"pre-closeout row observed result must equal PASS: {threat_id}")
            if identity != expected_head:
                fail(f"security {threat_id} commit identity must equal audited implementation head")
            if status != "mitigated":
                fail(f"pre-closeout row status must equal mitigated: {threat_id}")
    actual = set(by_id)
    expected = set(expected_threats)
    if actual != expected:
        fail(f"security threat set mismatch: missing={sorted(expected - actual)} extra={sorted(actual - expected)}")
    precloseout_open = sum(
        1 for threat_id, row in by_id.items()
        if not threat_id.startswith("T-31.1-20-") and row[7] != "mitigated"
    )
    postcloseout_pending = sum(
        1 for threat_id, row in by_id.items()
        if threat_id.startswith("T-31.1-20-") and row[7] == "pending_external_gate"
    )
    derived = {
        "high_threat_rows": str(len(rows)),
        "precloseout_high_threats_open": str(precloseout_open),
        "post_closeout_high_threats_pending": str(postcloseout_pending),
    }
    require_meta(meta, derived, "SECURITY")


def strip_table_lines(text: str, bounds: tuple[int, int]) -> str:
    lines = text.splitlines()
    start, end = bounds
    return "\n".join(lines[:start] + lines[end:])


def token_present(text: str, token: str) -> bool:
    return re.search(rf"(?<![A-Za-z0-9_.-]){re.escape(token)}(?![A-Za-z0-9_.-])", text) is not None


def verify_validation(text: str, expected_tasks: list[ExpectedTask], expected_head: str) -> None:
    meta = parse_frontmatter(text, "VALIDATION")
    require_meta(meta, EXPECTED_VALIDATION_META, "VALIDATION")
    if meta.get("audited_implementation_head") != expected_head:
        fail("VALIDATION audited_implementation_head must equal expected implementation head")
    for token in LEGACY_VALIDATION_TOKENS:
        if token in text:
            fail(f"legacy validation token is forbidden: {token}")
    rows, bounds = parse_section_table(text, "## Gap Task Validation Map", VALIDATION_HEADER, "VALIDATION")
    expected_by_task = {task.task_id: task for task in expected_tasks}
    expected_pairs = {
        (task.task_id, evidence_id)
        for task in expected_tasks
        for evidence_id in task.evidence_ids
    }
    actual_by_task: dict[str, tuple[str, ...]] = {}
    actual_pairs: set[tuple[str, str]] = set()
    for row in rows:
        task_id, plan_text, ordinal_text, evidence_text, result, commit = row
        if task_id in actual_by_task:
            fail(f"duplicate validation task id: {task_id}")
        actual_by_task[task_id] = row
        if not re.fullmatch(r"31\.1-\d{2}-\d{2}", task_id):
            fail(f"malformed validation task id: {task_id}")
        expected_task = expected_by_task.get(task_id)
        if expected_task is not None:
            if plan_text != f"{expected_task.plan:02d}":
                fail(f"validation {task_id} plan column mismatch")
            if ordinal_text != str(expected_task.ordinal):
                fail(f"validation {task_id} task ordinal mismatch")
        evidence_ids = [token.strip() for token in evidence_text.split(",") if token.strip()]
        if not evidence_ids:
            fail(f"validation {task_id} has no selector/assertion IDs")
        if len(evidence_ids) != len(set(evidence_ids)):
            fail(f"duplicate evidence id in validation cell: {task_id}")
        for evidence_id in evidence_ids:
            if not (
                re.fullmatch(r"Test[A-Za-z0-9_]+", evidence_id)
                or re.fullmatch(r"[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+", evidence_id)
            ):
                fail(f"validation {task_id} has malformed evidence id: {evidence_id}")
            actual_pairs.add((task_id, evidence_id))
        if result != "PASS":
            fail(f"validation {task_id} automated result must equal PASS")
        if commit != expected_head:
            fail(f"validation {task_id} audited commit must equal expected implementation head")
    actual_tasks = set(actual_by_task)
    expected_task_ids = set(expected_by_task)
    outside = strip_table_lines(text, bounds)
    for task_id in expected_task_ids:
        if token_present(outside, task_id):
            fail(f"normalized task id appears outside designated table: {task_id}")
    expected_evidence = {evidence_id for _, evidence_id in expected_pairs}
    for evidence_id in expected_evidence:
        if token_present(outside, evidence_id):
            fail(f"expected evidence id appears outside designated table: {evidence_id}")
    for match in re.finditer(r"(?<![A-Za-z0-9_.-])31\.1-(\d{2})-(\d{2})(?![A-Za-z0-9_.-])", outside):
        if int(match.group(1)) <= 10:
            fail(f"normalized task id appears outside designated table: {match.group(0)}")
    if actual_tasks != expected_task_ids:
        fail(f"validation task set mismatch: missing={sorted(expected_task_ids - actual_tasks)} extra={sorted(actual_tasks - expected_task_ids)}")
    if actual_pairs != expected_pairs:
        fail(f"validation evidence pair set mismatch: missing={sorted(expected_pairs - actual_pairs)} extra={sorted(actual_pairs - expected_pairs)}")
    derived = {
        "gap_plans": str(len({task.plan for task in expected_tasks})),
        "mapped_gap_tasks": str(len(actual_tasks)),
        "mapped_gap_evidence_pairs": str(len(actual_pairs)),
        "unique_gap_evidence_ids": str(len({evidence_id for _, evidence_id in actual_pairs})),
    }
    require_meta(meta, derived, "VALIDATION")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase-dir", type=Path, required=True)
    parser.add_argument("--expected-implementation-head", required=True)
    parser.add_argument("--security", type=Path, required=True)
    parser.add_argument("--validation", type=Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if re.fullmatch(r"[0-9a-f]{40}", args.expected_implementation_head) is None:
        fail("--expected-implementation-head must be exactly 40 lowercase hexadecimal characters")
    threats, tasks = derive_expected(args.phase_dir)
    verify_security(read_text(args.security), threats, args.expected_implementation_head)
    verify_validation(read_text(args.validation), tasks, args.expected_implementation_head)
    print(
        "[phase-31.1-audit] pass: "
        f"plans={len(PLAN_NUMBERS)} high_threats={len(threats)} tasks={len(tasks)} "
        f"task_evidence_pairs={sum(len(task.evidence_ids) for task in tasks)} "
        f"unique_evidence_ids={len({item for task in tasks for item in task.evidence_ids})} "
        f"implementation_head={args.expected_implementation_head}"
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except AuditError as exc:
        print(f"[phase-31.1-audit] error: {exc}", file=sys.stderr)
        raise SystemExit(1)
