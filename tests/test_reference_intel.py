from __future__ import annotations

import argparse
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT / "scripts"))

from reference_intel import pipeline  # noqa: E402


def sample_repository(local_path: str = "reference/sample") -> pipeline.Repository:
    return pipeline.Repository(
        local_name="sample",
        local_path=local_path,
        owner="example",
        name="sample",
        full_name="example/sample",
        origin="https://github.com/example/sample.git",
        html_url="https://github.com/example/sample",
        snapshot_sha="a" * 40,
    )


def sample_model_result(excerpt: str = "Feature X") -> dict:
    return {
        "record_classification": {
            "status": "implementation_bearing",
            "reason": "发布记录包含明确实现信号。",
            "confidence": 0.95,
        },
        "capabilities": [
            {
                "key": "feature-x",
                "name": "功能 X",
                "summary_zh": "提供功能 X。",
                "capability_type": "feature",
                "module": "relay_gateway",
                "implementation_status": "implemented",
                "user_visible": True,
                "confidence": 0.95,
                "evidence_excerpt": excerpt,
                "evidence_reason": "发布正文明确说明。",
                "version": "v1.0.0",
                "breaking_change": False,
                "security_relevant": False,
                "keywords": ["feature-x"],
            }
        ],
        "needs_review": False,
        "review_reason": "",
    }


class FakeCleaner:
    def __init__(self) -> None:
        self.calls = 0

    def clean(self, prompt: str, response_path: Path) -> dict:
        self.calls += 1
        self.last_prompt = prompt
        return sample_model_result()


class ReferenceIntelTests(unittest.TestCase):
    def test_parse_github_origin_supports_common_forms(self) -> None:
        expected = ("owner", "repo")
        self.assertEqual(pipeline.parse_github_origin("https://github.com/owner/repo.git"), expected)
        self.assertEqual(pipeline.parse_github_origin("git@github.com:owner/repo.git"), expected)
        self.assertEqual(pipeline.parse_github_origin("ssh://git@github.com/owner/repo.git"), expected)

    def test_changelog_discovery_rejects_source_history_files(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            project = root / "reference" / "sample"
            project.mkdir(parents=True)
            (project / "CHANGELOG.md").write_text("release notes", encoding="utf-8")
            (project / "history.js").write_text("browser history", encoding="utf-8")
            repo = sample_repository()
            found = [path.name for path in pipeline.find_changelog_files(root, repo)]
        self.assertEqual(found, ["CHANGELOG.md"])

    def test_github_client_streams_json_lines_and_honors_limit(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            fake_gh = Path(temporary) / "fake-gh"
            fake_gh.write_text(
                "#!/bin/sh\n"
                "printf '%s\\n' '{\"number\":1}' '{\"number\":2}' '{\"number\":3}'\n"
                "sleep 1\n",
                encoding="utf-8",
            )
            fake_gh.chmod(0o755)
            client = pipeline.GitHubClient(gh_bin=str(fake_gh), attempts=1, timeout=5)
            records = list(client.iter_all("repos/example/sample/issues?per_page=100", max_items=2))
        self.assertEqual([record["number"] for record in records], [1, 2])

    def test_issue_linked_from_merged_pr_receives_strong_evidence(self) -> None:
        repo = sample_repository()
        pulls = pipeline.normalize_pull_requests(
            repo,
            [
                {
                    "number": 8,
                    "title": "Feature X",
                    "body": "Fixes #12",
                    "merged_at": "2026-01-02T00:00:00Z",
                    "updated_at": "2026-01-02T00:00:00Z",
                    "merge_commit_sha": "b" * 40,
                    "html_url": "https://github.com/example/sample/pull/8",
                }
            ],
            "2026-01-03T00:00:00Z",
            None,
            1000,
        )
        issues = pipeline.normalize_issues(
            repo,
            [
                {
                    "number": 12,
                    "title": "Feature X request",
                    "body": "Please add Feature X",
                    "state": "closed",
                    "closed_at": "2026-01-02T00:00:00Z",
                    "updated_at": "2026-01-02T00:00:00Z",
                    "html_url": "https://github.com/example/sample/issues/12",
                }
            ],
            pulls,
            "2026-01-03T00:00:00Z",
            None,
            1000,
        )
        self.assertEqual(issues[0]["implementation_evidence"]["level"], "strong")
        self.assertEqual(issues[0]["source"]["linked_merged_pull_requests"][0]["number"], 8)

    def test_ungrounded_model_excerpt_cannot_enter_inventory(self) -> None:
        repo = sample_repository()
        record = pipeline.make_record(
            repo,
            "release",
            "1",
            "Feature X",
            "Feature X is available.",
            "https://github.com/example/sample/releases/tag/v1.0.0",
            "published",
            {"tag_name": "v1.0.0"},
            "2026-01-03T00:00:00Z",
        )
        unit = pipeline.make_source_units(record, 1000)[0]
        grounded = pipeline.ground_model_result(sample_model_result("invented evidence"), unit, "luna", 0.7)
        capability = grounded["capabilities"][0]
        self.assertFalse(capability["excerpt_grounded"])
        self.assertFalse(capability["accepted_for_inventory"])
        self.assertTrue(grounded["needs_review"])

    def test_codex_cleaner_sets_one_off_reasoning_effort(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cleaner = pipeline.CodexLunaCleaner(
                "/bin/true",
                "gpt-5.6-luna",
                REPO_ROOT / "scripts" / "reference_intel" / "feature_record.schema.json",
                root / "model-cwd",
                10,
                "low",
            )

            def fake_run(command: list[str], **_: object) -> subprocess.CompletedProcess[str]:
                output_path = Path(command[command.index("--output-last-message") + 1])
                output_path.write_text(json.dumps(sample_model_result()), encoding="utf-8")
                return subprocess.CompletedProcess(command, 0, "", "")

            with patch.object(pipeline.subprocess, "run", side_effect=fake_run) as mocked_run:
                cleaner.clean("prompt", root / "response.json")

            command = mocked_run.call_args.args[0]
            self.assertIn('model_reasoning_effort="low"', command)

    def test_reasoning_effort_change_invalidates_old_checkpoint(self) -> None:
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            repo = sample_repository()
            record = pipeline.make_record(
                repo,
                "release",
                "1",
                "Feature X",
                "Feature X is available.",
                "https://github.com/example/sample/releases/tag/v1.0.0",
                "published",
                {"tag_name": "v1.0.0"},
                "2026-01-03T00:00:00Z",
            )
            pipeline.atomic_write_jsonl(
                workdir / "raw" / "repos" / "example-sample" / "release.jsonl",
                [record],
            )
            pipeline.rebuild_raw_index(workdir)
            args = argparse.Namespace(
                repo_root=repo_root,
                workdir=workdir,
                clean_source=["release"],
                repo=[],
                max_prompt_chars=1000,
                model="luna-test",
                model_reasoning_effort="low",
                codex_bin="codex",
                model_timeout=10,
                max_attempts=1,
                sleep_seconds=0,
                progress_every=1,
                min_confidence=0.7,
                limit=0,
                max_clean_errors=1,
                retry_errors=False,
                allow_clean_errors=False,
            )
            low_cleaner = FakeCleaner()
            pipeline.clean_command(args, cleaner=low_cleaner)
            args.model_reasoning_effort = "high"
            high_cleaner = FakeCleaner()
            pipeline.clean_command(args, cleaner=high_cleaner)
            self.assertEqual(low_cleaner.calls, 1)
            self.assertEqual(high_cleaner.calls, 1)
            item = next((workdir / "clean" / "items").glob("*.json"))
            self.assertEqual(pipeline.read_json(item)["model_reasoning_effort"], "high")

            unit = pipeline.make_source_units(record, 1000)[0]
            stale_error = workdir / "clean" / "errors" / "stale.json"
            pipeline.atomic_write_json(
                stale_error,
                {
                    "source_unit_id": unit["source_unit_id"],
                    "source_content_sha256": record["content_sha256"],
                    "prompt_version": pipeline.PROMPT_VERSION,
                    "model": "luna-test",
                },
            )
            self.assertFalse(pipeline.error_is_current(stale_error, unit, "luna-test", "low"))

    def test_candidate_filter_keeps_feature_pr_and_drops_docs_only_pr(self) -> None:
        repo = sample_repository()
        feature = pipeline.make_record(
            repo,
            "pull_request",
            "8",
            "feat: add streaming retry",
            "Implement automatic retry for streamed responses.",
            "https://github.com/example/sample/pull/8",
            "merged",
            {"merge_commit_sha": "b" * 40, "merged_at": "2026-01-02T00:00:00Z", "labels": []},
            "2026-01-03T00:00:00Z",
        )
        docs = pipeline.make_record(
            repo,
            "pull_request",
            "9",
            "docs: update README",
            "Update wording only.",
            "https://github.com/example/sample/pull/9",
            "merged",
            {"merge_commit_sha": "c" * 40, "merged_at": "2026-01-02T00:00:00Z", "labels": []},
            "2026-01-03T00:00:00Z",
        )
        self.assertTrue(pipeline.is_implementation_candidate(feature))
        self.assertFalse(pipeline.is_implementation_candidate(docs))

    def test_clean_resume_and_aggregate_preserve_provenance(self) -> None:
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            repo = sample_repository()
            record = pipeline.make_record(
                repo,
                "release",
                "1",
                "Feature X",
                "Feature X is available.",
                "https://github.com/example/sample/releases/tag/v1.0.0",
                "published",
                {"tag_name": "v1.0.0"},
                "2026-01-03T00:00:00Z",
            )
            pipeline.atomic_write_jsonl(
                workdir / "raw" / "repos" / "example-sample" / "release.jsonl",
                [record],
            )
            pipeline.rebuild_raw_index(workdir)
            args = argparse.Namespace(
                repo_root=repo_root,
                workdir=workdir,
                clean_source=["release"],
                repo=[],
                max_prompt_chars=1000,
                model="luna-test",
                codex_bin="codex",
                model_timeout=10,
                max_attempts=1,
                sleep_seconds=0,
                progress_every=1,
                min_confidence=0.7,
                limit=0,
                max_clean_errors=1,
                retry_errors=False,
                allow_clean_errors=False,
            )
            first = FakeCleaner()
            pipeline.clean_command(args, cleaner=first)
            second = FakeCleaner()
            pipeline.clean_command(args, cleaner=second)
            catalog = pipeline.aggregate_command(
                argparse.Namespace(
                    repo_root=repo_root,
                    workdir=workdir,
                    min_confidence=0.7,
                    include_fixes=True,
                )
            )
        self.assertEqual(first.calls, 1)
        self.assertEqual(second.calls, 0)
        self.assertEqual(catalog["coverage"]["accepted_groups"], 1)
        self.assertEqual(catalog["features"][0]["evidence"][0]["source_content_sha256"], record["content_sha256"])
        self.assertEqual(catalog["features"][0]["evidence"][0]["model_reasoning_effort"], "low")


if __name__ == "__main__":
    unittest.main()
