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
    def __init__(self, result: dict | None = None) -> None:
        self.calls = 0
        self.result = result or sample_model_result()

    def clean(self, prompt: str, response_path: Path) -> dict:
        self.calls += 1
        self.last_prompt = prompt
        return self.result


class ReferenceIntelTests(unittest.TestCase):
    def test_catalog_default_confidence_is_precision_oriented(self) -> None:
        args = pipeline.build_parser().parse_args(["aggregate", "--workdir", "/tmp/reference-intel"])
        self.assertEqual(args.min_confidence, 0.80)

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
        self.assertEqual(
            issues[0]["source"]["linked_merged_pull_requests"][0]["title"], "Feature X"
        )
        self.assertEqual(
            issues[0]["implementation_evidence"]["linked_merged_pull_requests"][0]["record_id"],
            pulls[0]["record_id"],
        )
        unit = pipeline.make_source_units(issues[0], 1000)[0]
        self.assertIn("LINKED_MERGED_PULL_REQUEST_8_START", unit["evidence_chunk"])
        grounded = pipeline.ground_model_result(
            sample_model_result("Fixes #12"), unit, "luna", 0.7
        )
        self.assertTrue(grounded["capabilities"][0]["accepted_for_inventory"])
        marker = pipeline.ground_model_result(
            sample_model_result("LINKED_MERGED_PULL_REQUEST_8_START"), unit, "luna", 0.7
        )
        self.assertFalse(marker["capabilities"][0]["excerpt_grounded"])

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

    def test_title_excerpt_can_enter_inventory(self) -> None:
        repo = sample_repository()
        record = pipeline.make_record(
            repo,
            "release",
            "1",
            "Title-only implementation evidence",
            "No repeated title in this body.",
            "https://github.com/example/sample/releases/tag/v1.0.0",
            "published",
            {"tag_name": "v1.0.0"},
            "2026-01-03T00:00:00Z",
        )
        unit = pipeline.make_source_units(record, 1000)[0]
        grounded = pipeline.ground_model_result(
            sample_model_result("Title-only implementation evidence"), unit, "luna", 0.7
        )
        self.assertTrue(grounded["capabilities"][0]["excerpt_grounded"])
        self.assertTrue(grounded["capabilities"][0]["accepted_for_inventory"])

    def test_release_license_metadata_cannot_enter_inventory(self) -> None:
        repo = sample_repository()
        record = pipeline.make_record(
            repo,
            "release",
            "2",
            "OpenAI OAuth v2",
            "Apache-2.0 License\nNew feature notes.",
            "https://github.com/example/sample/releases/tag/v2.0.0",
            "published",
            {"tag_name": "v2.0.0"},
            "2026-01-03T00:00:00Z",
        )
        unit = pipeline.make_source_units(record, 1000)[0]
        result = sample_model_result("Apache-2.0 License")
        result["capabilities"][0]["name"] = "采用 Apache-2.0 许可"
        result["capabilities"][0]["summary_zh"] = "项目采用 Apache-2.0 许可。"
        grounded = pipeline.ground_model_result(result, unit, "luna", 0.7)
        self.assertFalse(grounded["capabilities"][0]["accepted_for_inventory"])

        refactor_record = pipeline.make_record(
            repo,
            "release",
            "3",
            "v3.0.0",
            "Move database to packages.",
            "https://github.com/example/sample/releases/tag/v3.0.0",
            "published",
            {"tag_name": "v3.0.0"},
            "2026-01-03T00:00:00Z",
        )
        refactor_result = sample_model_result("Move database to packages")
        refactor_result["capabilities"][0].update(
            {
                "capability_type": "other",
                "user_visible": False,
                "keywords": ["database", "refactoring"],
            }
        )
        refactor_grounded = pipeline.ground_model_result(
            refactor_result,
            pipeline.make_source_units(refactor_record, 1000)[0],
            "luna",
            0.7,
        )
        self.assertFalse(refactor_grounded["capabilities"][0]["accepted_for_inventory"])

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
            stale_item_value = pipeline.read_json(item)
            stale_item_value["schema_version"] = "oblivious-reference-intel/clean/v1"
            pipeline.atomic_write_json(item, stale_item_value)
            self.assertFalse(pipeline.result_is_current(item, unit, "luna-test", "high"))
            stale_error = workdir / "clean" / "errors" / "stale.json"
            pipeline.atomic_write_json(
                stale_error,
                {
                    "schema_version": "oblivious-reference-intel/clean/v1",
                    "source_unit_id": unit["source_unit_id"],
                    "source_content_sha256": record["content_sha256"],
                    "prompt_version": pipeline.PROMPT_VERSION,
                    "model": "luna-test",
                },
            )
            self.assertFalse(pipeline.error_is_current(stale_error, unit, "luna-test", "low"))

    def test_aggregate_rejects_incomplete_clean_input_by_default(self) -> None:
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
            pipeline.atomic_write_jsonl(workdir / "clean" / "records.jsonl", [])
            pipeline.atomic_write_json(
                workdir / "clean" / "manifest.json",
                {
                    "schema_version": pipeline.CLEAN_SCHEMA_VERSION,
                    "prompt_version": pipeline.PROMPT_VERSION,
                    "model": "luna-test",
                    "model_reasoning_effort": "low",
                    "complete": False,
                    "counts": {
                        "source_units": 1,
                        "successful_units": 0,
                        "error_units": 0,
                        "pending_units": 1,
                        "indexed_units": 0,
                    },
                },
            )
            args = argparse.Namespace(
                repo_root=repo_root,
                workdir=workdir,
                min_confidence=0.7,
                include_fixes=True,
                allow_incomplete_clean=False,
            )
            with self.assertRaisesRegex(pipeline.PipelineError, "Cleaning is incomplete"):
                pipeline.aggregate_command(args)

            args.allow_incomplete_clean = True
            catalog = pipeline.aggregate_command(args)

        self.assertFalse(catalog["clean_input"]["complete"])
        self.assertTrue(catalog["clean_input"]["diagnostic_incomplete_override"])
        self.assertEqual(catalog["coverage"]["accepted_claims"], 0)

    def test_status_marks_stale_clean_contract_as_incomplete(self) -> None:
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            pipeline.atomic_write_json(
                workdir / "raw" / "manifest.json",
                {"counts": {"release": 1}, "failures": []},
            )
            pipeline.atomic_write_jsonl(workdir / "raw" / "records.jsonl", [])
            pipeline.atomic_write_json(
                workdir / "clean" / "manifest.json",
                {
                    "schema_version": "oblivious-reference-intel/clean/v1",
                    "prompt_version": "reference-feature-cleaner/v1",
                    "model": "gpt-5.4-mini",
                    "model_reasoning_effort": "low",
                    "complete": True,
                    "counts": {
                        "source_units": 1,
                        "successful_units": 1,
                        "error_units": 0,
                        "pending_units": 0,
                        "indexed_units": 1,
                    },
                },
            )
            pipeline.atomic_write_json(
                workdir / "clean" / "progress.json",
                {
                    "schema_version": "oblivious-reference-intel/clean/v1",
                    "prompt_version": "reference-feature-cleaner/v1",
                    "model": "gpt-5.4-mini",
                    "model_reasoning_effort": "low",
                    "running": True,
                    "started_at": "2026-01-03T00:00:00Z",
                    "updated_at": "2026-01-03T00:01:00Z",
                    "sources": ["release"],
                    "repositories": [],
                    "implementation_candidates_only": True,
                    "max_prompt_chars": 1000,
                    "min_confidence": 0.8,
                    "source_records": 1,
                    "source_units": 1,
                    "last_position": 1,
                },
            )
            with patch("builtins.print"):
                status = pipeline.status_command(
                    argparse.Namespace(repo_root=repo_root, workdir=workdir)
                )

        self.assertFalse(status["clean_contract_current"])
        self.assertFalse(status["clean_complete"])
        self.assertEqual(status["clean_schema_version"], "oblivious-reference-intel/clean/v1")
        self.assertEqual(status["expected_clean_schema_version"], pipeline.CLEAN_SCHEMA_VERSION)
        self.assertEqual(status["expected_clean_prompt_version"], pipeline.PROMPT_VERSION)
        self.assertFalse(status["clean_progress_current"])
        self.assertEqual(status["clean_progress"], {})

    def test_candidate_filter_keeps_neutral_pr_and_drops_deterministic_non_features(self) -> None:
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
        tests_only = pipeline.make_record(
            repo,
            "pull_request",
            "10",
            "test: stop Bedrock tests from making real network calls",
            "Inject a mock HTTP client, fix CI hangs, and assert the request body. This PR only changes tests.",
            "https://github.com/example/sample/pull/10",
            "merged",
            {"merge_commit_sha": "d" * 40, "merged_at": "2026-01-02T00:00:00Z", "labels": []},
            "2026-01-03T00:00:00Z",
        )
        neutral = pipeline.make_record(
            repo,
            "pull_request",
            "11",
            "Handle cancelled streams gracefully",
            "Prevents duplicate completion after a cancelled response.",
            "https://github.com/example/sample/pull/11",
            "merged",
            {"merge_commit_sha": "e" * 40, "merged_at": "2026-01-02T00:00:00Z", "labels": []},
            "2026-01-03T00:00:00Z",
        )
        self.assertTrue(pipeline.is_implementation_candidate(feature))
        self.assertTrue(pipeline.is_implementation_candidate(neutral))
        self.assertFalse(pipeline.is_implementation_candidate(docs))
        self.assertFalse(pipeline.is_implementation_candidate(tests_only))

        unit = pipeline.make_source_units(tests_only, 1000)[0]
        grounded = pipeline.ground_model_result(
            sample_model_result("stop Bedrock tests from making real network calls"),
            unit,
            "luna",
            0.7,
        )
        self.assertFalse(grounded["capabilities"][0]["accepted_for_inventory"])

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
            progress = pipeline.read_json(workdir / "clean" / "progress.json")
            with patch("builtins.print"):
                status = pipeline.status_command(
                    argparse.Namespace(repo_root=repo_root, workdir=workdir)
                )
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
        self.assertTrue(pipeline.clean_progress_contract_current(progress))
        self.assertFalse(progress["running"])
        self.assertEqual(progress["last_position"], 1)
        self.assertEqual(progress["final_counts"]["successful_units"], 1)
        self.assertTrue(status["clean_progress_current"])
        self.assertEqual(status["clean_progress"]["model"], "luna-test")
        self.assertEqual(catalog["coverage"]["accepted_groups"], 1)
        self.assertEqual(catalog["features"][0]["evidence"][0]["source_content_sha256"], record["content_sha256"])
        self.assertEqual(catalog["features"][0]["evidence"][0]["model_reasoning_effort"], "low")

    def test_review_required_claim_is_held_out_of_catalog(self) -> None:
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
            model_result = sample_model_result()
            model_result["needs_review"] = True
            model_result["review_reason"] = "发布条目过于宽泛。"
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
            pipeline.clean_command(args, cleaner=FakeCleaner(model_result))
            catalog = pipeline.aggregate_command(
                argparse.Namespace(
                    repo_root=repo_root,
                    workdir=workdir,
                    min_confidence=0.7,
                    include_fixes=True,
                )
            )
            review = list(pipeline.iter_jsonl(workdir / "catalog" / "review-queue.jsonl"))

        self.assertEqual(catalog["coverage"]["accepted_claims"], 0)
        self.assertEqual(catalog["coverage"]["review_units"], 1)
        self.assertEqual(catalog["coverage"]["review_claims"], 1)
        self.assertEqual(len(review[0]["candidate_claims"]), 1)

    def test_materialize_sample_enriches_issue_with_linked_pr_content(self) -> None:
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            work_root = Path(work_temporary)
            source_workdir = work_root / "full"
            sample_workdir = work_root / "sample"
            repo = sample_repository()
            pull = pipeline.normalize_pull_requests(
                repo,
                [
                    {
                        "number": 8,
                        "title": "feat: implement Feature X",
                        "body": "Fixes #12 by adding Feature X.",
                        "merged_at": "2026-01-02T00:00:00Z",
                        "updated_at": "2026-01-02T00:00:00Z",
                        "merge_commit_sha": "b" * 40,
                        "html_url": "https://github.com/example/sample/pull/8",
                    }
                ],
                "2026-01-03T00:00:00Z",
                None,
                1000,
            )[0]
            issue = pipeline.make_record(
                repo,
                "issue",
                "12",
                "Feature X request",
                "Please add Feature X.",
                "https://github.com/example/sample/issues/12",
                "closed",
                {
                    "number": 12,
                    "closed_at": "2026-01-02T00:00:00Z",
                    "linked_merged_pull_requests": [
                        {
                            "number": 8,
                            "url": pull["url"],
                            "merge_commit_sha": pull["source"]["merge_commit_sha"],
                            "merged_at": pull["source"]["merged_at"],
                        }
                    ],
                },
                "2026-01-03T00:00:00Z",
            )
            raw_dir = source_workdir / "raw" / "repos" / "example-sample"
            pipeline.atomic_write_jsonl(raw_dir / "issue.jsonl", [issue])
            pipeline.atomic_write_jsonl(raw_dir / "pull_request.jsonl", [pull])
            pipeline.rebuild_raw_index(source_workdir)
            selection_path = work_root / "selection.json"
            pipeline.atomic_write_json(
                selection_path,
                {
                    "schema_version": "oblivious-reference-intel/sample/v1",
                    "sample_only": True,
                    "selections": [
                        {
                            "record_id": issue["record_id"],
                            "repository": repo.full_name,
                            "kind": "issue",
                            "content_sha256": issue["content_sha256"],
                            "source_url": issue["url"],
                            "unit_count": 1,
                        }
                    ],
                },
            )
            pipeline.materialize_sample_command(
                argparse.Namespace(
                    repo_root=repo_root,
                    source_workdir=source_workdir,
                    workdir=sample_workdir,
                    selection_manifest=selection_path,
                    max_prompt_chars=1000,
                )
            )
            materialized = next(pipeline.iter_jsonl(sample_workdir / "raw" / "records.jsonl"))
            sample_manifest = pipeline.read_json(sample_workdir / "sample-selection.json")

        linked = materialized["source"]["linked_merged_pull_requests"][0]
        self.assertEqual(linked["record_id"], pull["record_id"])
        self.assertEqual(linked["content_sha256"], pull["content_sha256"])
        self.assertEqual(linked["title"], "feat: implement Feature X")
        self.assertIn("adding Feature X", linked["body"])
        self.assertNotEqual(materialized["content_sha256"], issue["content_sha256"])
        self.assertEqual(sample_manifest["selected_records"], 1)
        self.assertEqual(sample_manifest["linked_pr_context_records"], 1)

    def test_materialize_full_corpus_preserves_source_and_reports_both_scopes(self) -> None:
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            work_root = Path(work_temporary)
            source_workdir = work_root / "full-v1"
            corpus_workdir = work_root / "full-v3"
            repo = sample_repository()
            neutral_pull = pipeline.make_record(
                repo,
                "pull_request",
                "8",
                "Handle cancelled streams gracefully",
                "Prevents duplicate completion after a cancelled response.",
                "https://github.com/example/sample/pull/8",
                "merged",
                {
                    "number": 8,
                    "merge_commit_sha": "b" * 40,
                    "merged_at": "2026-01-02T00:00:00Z",
                    "labels": [],
                },
                "2026-01-03T00:00:00Z",
            )
            docs_pull = pipeline.make_record(
                repo,
                "pull_request",
                "9",
                "docs: clarify stream cancellation",
                "Documentation wording only.",
                "https://github.com/example/sample/pull/9",
                "merged",
                {
                    "number": 9,
                    "merge_commit_sha": "c" * 40,
                    "merged_at": "2026-01-02T00:00:00Z",
                    "labels": [],
                },
                "2026-01-03T00:00:00Z",
            )
            linked_issue = pipeline.make_record(
                repo,
                "issue",
                "12",
                "Cancelled stream completion",
                "A cancelled stream can complete twice.",
                "https://github.com/example/sample/issues/12",
                "closed",
                {
                    "number": 12,
                    "closed_at": "2026-01-02T00:00:00Z",
                    "linked_merged_pull_requests": [
                        {
                            "number": 8,
                            "url": neutral_pull["url"],
                            "merge_commit_sha": neutral_pull["source"]["merge_commit_sha"],
                            "merged_at": neutral_pull["source"]["merged_at"],
                        }
                    ],
                },
                "2026-01-03T00:00:00Z",
            )
            weak_issue = pipeline.make_record(
                repo,
                "issue",
                "13",
                "Add another stream mode",
                "This request has no merged implementation evidence.",
                "https://github.com/example/sample/issues/13",
                "closed",
                {
                    "number": 13,
                    "closed_at": "2026-01-02T00:00:00Z",
                    "linked_merged_pull_requests": [],
                },
                "2026-01-03T00:00:00Z",
            )
            release = pipeline.make_record(
                repo,
                "release",
                "1",
                "v1.0.0",
                "Cancelled streams now finish exactly once.",
                "https://github.com/example/sample/releases/tag/v1.0.0",
                "published",
                {"tag_name": "v1.0.0"},
                "2026-01-03T00:00:00Z",
            )
            changelog = pipeline.make_record(
                repo,
                "changelog",
                "CHANGELOG.md",
                "CHANGELOG.md",
                "Added cancellation handling.",
                "https://github.com/example/sample/blob/main/CHANGELOG.md",
                "tracked",
                {"path": "CHANGELOG.md", "snapshot_sha": "a" * 40},
                "2026-01-03T00:00:00Z",
            )
            tag = pipeline.make_record(
                repo,
                "tag",
                "v1.0.0",
                "v1.0.0",
                "",
                "https://github.com/example/sample/releases/tag/v1.0.0",
                "published",
                {"tag_name": "v1.0.0"},
                "2026-01-03T00:00:00Z",
            )
            source_records = [
                linked_issue,
                weak_issue,
                neutral_pull,
                docs_pull,
                release,
                changelog,
                tag,
            ]
            raw_dir = source_workdir / "raw" / "repos" / "example-sample"
            for kind in ("issue", "pull_request", "release", "changelog", "tag"):
                pipeline.atomic_write_jsonl(
                    raw_dir / f"{kind}.jsonl",
                    [record for record in source_records if record["kind"] == kind],
                )
            pipeline.atomic_write_json(raw_dir / "repository.json", repo.as_dict())
            source_index, source_counts = pipeline.rebuild_raw_index(source_workdir)
            source_manifest_path = source_workdir / "raw" / "manifest.json"
            pipeline.atomic_write_json(
                source_manifest_path,
                {
                    "schema_version": pipeline.RAW_SCHEMA_VERSION,
                    "repositories": [repo.as_dict()],
                    "counts": dict(sorted(source_counts.items())),
                    "failures": [],
                    "records_path": str(source_index),
                },
            )
            source_manifest_sha = pipeline.sha256_file(source_manifest_path)
            source_index_sha = pipeline.sha256_file(source_index)
            source_by_id = {record["record_id"]: record for record in source_records}

            args = pipeline.build_parser().parse_args(
                [
                    "materialize-corpus",
                    "--repo-root",
                    str(repo_root),
                    "--source-workdir",
                    str(source_workdir),
                    "--workdir",
                    str(corpus_workdir),
                ]
            )
            corpus_manifest = args.handler(args)
            materialized_records = list(
                pipeline.iter_jsonl(corpus_workdir / "raw" / "records.jsonl")
            )
            materialized_by_id = {
                record["record_id"]: record for record in materialized_records
            }
            materialization = pipeline.read_json(
                corpus_workdir / "corpus-materialization.json"
            )
            with patch("builtins.print"):
                status = pipeline.status_command(
                    argparse.Namespace(repo_root=repo_root, workdir=corpus_workdir)
                )

            self.assertEqual(pipeline.sha256_file(source_manifest_path), source_manifest_sha)
            self.assertEqual(pipeline.sha256_file(source_index), source_index_sha)

        self.assertEqual(len(materialized_records), len(source_records))
        self.assertEqual(corpus_manifest["counts"], dict(sorted(source_counts.items())))
        self.assertEqual(materialization["record_count"], len(source_records))
        self.assertEqual(materialization["linked_pr_context_records"], 1)
        linked = materialized_by_id[linked_issue["record_id"]]["source"][
            "linked_merged_pull_requests"
        ][0]
        self.assertEqual(linked["record_id"], neutral_pull["record_id"])
        self.assertEqual(linked["content_sha256"], neutral_pull["content_sha256"])
        self.assertIn("duplicate completion", linked["body"])
        self.assertNotEqual(
            materialized_by_id[linked_issue["record_id"]]["content_sha256"],
            linked_issue["content_sha256"],
        )
        self.assertEqual(
            materialized_by_id[weak_issue["record_id"]]["content_sha256"],
            weak_issue["content_sha256"],
        )
        for record_id, source_record in source_by_id.items():
            if source_record["kind"] != "issue":
                self.assertEqual(
                    materialized_by_id[record_id]["content_sha256"],
                    source_record["content_sha256"],
                )
        self.assertEqual(
            status["unfiltered_cleanable_records"],
            {"changelog": 1, "issue": 2, "pull_request": 2, "release": 1},
        )
        self.assertEqual(status["unfiltered_cleanable_units"], 6)
        self.assertEqual(
            status["implementation_candidate_records"],
            {"changelog": 1, "issue": 1, "pull_request": 1, "release": 1},
        )
        self.assertEqual(status["implementation_candidate_units"], 4)
        self.assertFalse(status["clean_progress_current"])
        self.assertEqual(status["clean_progress"], {})

    def _seed_release_corpus(self, workdir: Path, count: int) -> None:
        repo = sample_repository()
        records = [
            pipeline.make_record(
                repo,
                "release",
                str(index),
                f"Feature {index}",
                f"Feature {index} is available.",
                f"https://github.com/example/sample/releases/tag/v{index}.0.0",
                "published",
                {"tag_name": f"v{index}.0.0"},
                "2026-01-03T00:00:00Z",
            )
            for index in range(1, count + 1)
        ]
        pipeline.atomic_write_jsonl(
            workdir / "raw" / "repos" / "example-sample" / "release.jsonl", records
        )
        pipeline.rebuild_raw_index(workdir)

    @staticmethod
    def _clean_args(repo_root: Path, workdir: Path, **overrides) -> argparse.Namespace:
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
            clean_workers=1,
            sleep_seconds=0,
            progress_every=1,
            min_confidence=0.7,
            limit=0,
            max_clean_errors=0,
            retry_errors=False,
            allow_clean_errors=False,
            implementation_candidates_only=False,
        )
        for key, value in overrides.items():
            setattr(args, key, value)
        return args

    def test_concurrent_clean_matches_sequential_checkpoints(self) -> None:
        """Concurrent cleaning must produce the same item set and counts as sequential."""

        class ThreadSafeFakeCleaner:
            def __init__(self) -> None:
                self.lock = __import__("threading").Lock()
                self.calls = 0
                self.max_in_flight = 0
                self.in_flight = 0

            def clean(self, prompt: str, response_path: Path) -> dict:
                with self.lock:
                    self.calls += 1
                    self.in_flight += 1
                    self.max_in_flight = max(self.max_in_flight, self.in_flight)
                excerpt = prompt.split("Feature ")[-1].split(" ")[0].strip()
                try:
                    __import__("time").sleep(0.02)
                    return sample_model_result(f"Feature {excerpt}")
                finally:
                    with self.lock:
                        self.in_flight -= 1

        sequential_items: set[str] = set()
        concurrent_items: set[str] = set()
        for workers, sink in ((1, sequential_items), (4, concurrent_items)):
            with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
                repo_root = Path(repo_temporary)
                workdir = Path(work_temporary)
                self._seed_release_corpus(workdir, 12)
                args = self._clean_args(repo_root, workdir, clean_workers=workers)
                cleaner = ThreadSafeFakeCleaner()
                manifest = pipeline.clean_command(args, cleaner=cleaner)

                self.assertEqual(cleaner.calls, 12)
                self.assertEqual(manifest["counts"]["successful_units"], 12)
                self.assertEqual(manifest["counts"]["error_units"], 0)
                self.assertEqual(manifest["counts"]["pending_units"], 0)
                self.assertTrue(manifest["complete"])
                self.assertEqual(manifest["clean_workers"], workers)
                self.assertEqual(manifest["sequential"], workers == 1)

                progress = pipeline.read_json(workdir / "clean" / "progress.json")
                self.assertFalse(progress["running"])
                self.assertEqual(progress["clean_workers"], workers)
                self.assertEqual(progress["successful_calls_this_run"], 12)
                self.assertEqual(progress["failed_calls_this_run"], 0)
                self.assertTrue(pipeline.clean_progress_contract_current(progress))

                if workers == 1:
                    self.assertEqual(cleaner.max_in_flight, 1)
                else:
                    self.assertGreater(cleaner.max_in_flight, 1)
                    self.assertLessEqual(cleaner.max_in_flight, workers)

                for item in (workdir / "clean" / "items").glob("*.json"):
                    sink.add(pipeline.read_json(item)["source_unit_id"])

        self.assertEqual(len(sequential_items), 12)
        self.assertEqual(sequential_items, concurrent_items)

    def test_concurrent_clean_resumes_and_respects_limit(self) -> None:
        """A bounded concurrent run honours --limit and reuses prior checkpoints."""
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            self._seed_release_corpus(workdir, 10)

            first = FakeCleaner()
            pipeline.clean_command(
                self._clean_args(
                    repo_root, workdir, clean_workers=3, limit=4, allow_clean_errors=True
                ),
                cleaner=first,
            )
            self.assertGreaterEqual(first.calls, 4)
            self.assertLessEqual(first.calls, 4 + 3)
            done_after_first = len(list((workdir / "clean" / "items").glob("*.json")))
            self.assertEqual(done_after_first, first.calls)

            second = FakeCleaner()
            manifest = pipeline.clean_command(
                self._clean_args(repo_root, workdir, clean_workers=3), cleaner=second
            )
            self.assertEqual(second.calls, 10 - done_after_first)
            self.assertEqual(manifest["counts"]["successful_units"], 10)
            self.assertTrue(manifest["complete"])

            third = FakeCleaner()
            pipeline.clean_command(
                self._clean_args(repo_root, workdir, clean_workers=3), cleaner=third
            )
            self.assertEqual(third.calls, 0)

    def test_concurrent_clean_records_errors_without_stale_heartbeat(self) -> None:
        """Concurrent failures are checkpointed and never leave running=true behind."""

        class FailingCleaner:
            def __init__(self) -> None:
                self.lock = __import__("threading").Lock()
                self.calls = 0

            def clean(self, prompt: str, response_path: Path) -> dict:
                with self.lock:
                    self.calls += 1
                raise pipeline.PipelineError("Luna call failed: token=SECRETVALUE upstream 500")

        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            self._seed_release_corpus(workdir, 6)
            args = self._clean_args(
                repo_root, workdir, clean_workers=3, allow_clean_errors=True
            )
            cleaner = FailingCleaner()
            manifest = pipeline.clean_command(args, cleaner=cleaner)

            self.assertEqual(cleaner.calls, 6)
            self.assertEqual(manifest["counts"]["successful_units"], 0)
            self.assertEqual(manifest["counts"]["error_units"], 6)
            self.assertFalse(manifest["complete"])

            progress = pipeline.read_json(workdir / "clean" / "progress.json")
            self.assertFalse(progress["running"])
            self.assertEqual(progress["failed_calls_this_run"], 6)
            self.assertEqual(
                progress["calls_this_run"],
                progress["successful_calls_this_run"] + progress["failed_calls_this_run"],
            )

            errors = list((workdir / "clean" / "errors").glob("*.json"))
            self.assertEqual(len(errors), 6)
            for error_path in errors:
                self.assertNotIn("SECRETVALUE", error_path.read_text(encoding="utf-8"))

    def test_product_rollup_drops_ungrounded_citations(self) -> None:
        """A synthesized feature may only cite claim keys that actually exist."""
        allowed = {
            "real-a": {
                "key": "real-a",
                "versions": ["v1.0.0"],
                "source_url": "https://example.test/a",
            },
            "real-b": {"key": "real-b", "versions": [], "source_url": None},
            "real-c": {"key": "real-c", "versions": ["v2.0.0"], "source_url": None},
        }
        raw = {
            "product_features": [
                {
                    "key": "kept",
                    "name": "保留的能力",
                    "summary_zh": "摘要",
                    "capability_scope": "core",
                    "user_value_zh": "价值",
                    "maturity": "established",
                    "user_visible": True,
                    "security_relevant": False,
                    # one real key, one hallucinated key
                    "source_claim_keys": ["real-a", "totally-invented"],
                    "representative_versions": ["v9.9.9-model-invented"],
                },
                {
                    "key": "dropped-entirely",
                    "name": "凭空生成的能力",
                    "summary_zh": "摘要",
                    "capability_scope": "core",
                    "user_value_zh": "价值",
                    "maturity": "introduced",
                    "user_visible": True,
                    "security_relevant": False,
                    "source_claim_keys": ["ghost-1", "ghost-2"],
                    "representative_versions": [],
                },
            ],
            "dropped_claim_keys": ["real-b", "not-a-real-key"],
            "notes": "备注",
        }
        grounded = pipeline.ground_product_rollup(
            pipeline.validate_product_rollup(raw), "example/sample", "chat", allowed
        )
        # The all-hallucinated feature is gone; the partially grounded one survives
        # with only its resolvable citation.
        self.assertEqual([f["key"] for f in grounded["product_features"]], ["kept"])
        kept = grounded["product_features"][0]
        self.assertEqual(kept["source_claim_keys"], ["real-a"])
        self.assertEqual(kept["claim_count"], 1)
        # Versions come from deterministic claim records, not the model's invention.
        self.assertEqual(kept["representative_versions"], ["v1.0.0"])
        self.assertEqual(kept["source_urls"], ["https://example.test/a"])
        self.assertEqual(grounded["ungrounded_citation_count"], 3)
        self.assertEqual(grounded["dropped_claim_keys"], ["real-b"])
        self.assertEqual(grounded["uncited_claim_keys"], ["real-c"])
        self.assertEqual(grounded["input_claim_count"], 3)

    def test_landscape_cannot_overstate_product_coverage(self) -> None:
        """Coverage counts are recomputed from grounded entries, never trusted."""
        allowed = {
            "example/one": {"feat-x"},
            "example/two": {"feat-y"},
        }
        raw = {
            "baseline_features": [
                {
                    "key": "shared",
                    "name": "共有能力",
                    "summary_zh": "摘要",
                    "commonality": "table_stakes",
                    "capability_scope": "core",
                    "user_visible": True,
                    "security_relevant": False,
                    "product_coverage": [
                        {
                            "repository": "example/one",
                            "product_feature_keys": ["feat-x"],
                            "variant_zh": "做法一",
                        },
                        {
                            "repository": "example/two",
                            "product_feature_keys": ["feat-y"],
                            "variant_zh": "做法二",
                        },
                        # Product that was never in the input at all.
                        {
                            "repository": "example/never-seen",
                            "product_feature_keys": ["feat-z"],
                            "variant_zh": "凭空生成",
                        },
                        # Real product, but a feature key it does not own.
                        {
                            "repository": "example/one",
                            "product_feature_keys": ["feat-y"],
                            "variant_zh": "错配",
                        },
                    ],
                },
                {
                    "key": "all-invented",
                    "name": "凭空能力",
                    "summary_zh": "摘要",
                    "commonality": "niche",
                    "capability_scope": "supporting",
                    "user_visible": False,
                    "security_relevant": False,
                    "product_coverage": [
                        {
                            "repository": "example/ghost",
                            "product_feature_keys": ["ghost"],
                            "variant_zh": "凭空",
                        }
                    ],
                },
            ],
            "unmatched_product_feature_keys": [],
            "notes": "备注",
        }
        grounded = pipeline.ground_landscape(
            pipeline.validate_landscape(raw), "chat", allowed
        )
        self.assertEqual([f["key"] for f in grounded["baseline_features"]], ["shared"])
        shared = grounded["baseline_features"][0]
        # 2 grounded products, not the 4 the model claimed.
        self.assertEqual(shared["product_count"], 2)
        self.assertEqual(
            [entry["repository"] for entry in shared["product_coverage"]],
            ["example/one", "example/two"],
        )
        self.assertEqual(grounded["ungrounded_coverage_count"], 3)
        self.assertEqual(grounded["input_product_count"], 2)
        self.assertEqual(grounded["input_feature_count"], 2)
        # Commonality is derived from grounded coverage, not the model's label.
        self.assertEqual(shared["coverage_ratio"], 1.0)
        self.assertEqual(shared["commonality"], "table_stakes")
        self.assertEqual(shared["commonality_model"], "table_stakes")

    def test_commonality_is_derived_not_trusted(self) -> None:
        """A model label that contradicts grounded coverage is overridden."""
        self.assertEqual(pipeline.derive_commonality(1, 30), "niche")
        self.assertEqual(pipeline.derive_commonality(3, 30), "differentiator")
        self.assertEqual(pipeline.derive_commonality(9, 30), "common")
        self.assertEqual(pipeline.derive_commonality(20, 30), "table_stakes")
        self.assertEqual(pipeline.derive_commonality(0, 30), "niche")
        self.assertEqual(pipeline.derive_commonality(1, 0), "niche")

        allowed = {"example/one": {"feat-x"}}
        raw = {
            "baseline_features": [
                {
                    "key": "solo",
                    "name": "单产品能力",
                    "summary_zh": "摘要",
                    # Model overstates adoption; only one product actually has it.
                    "commonality": "table_stakes",
                    "capability_scope": "core",
                    "user_visible": True,
                    "security_relevant": False,
                    "product_coverage": [
                        {
                            "repository": "example/one",
                            "product_feature_keys": ["feat-x"],
                            "variant_zh": "做法",
                        }
                    ],
                }
            ],
            "unmatched_product_feature_keys": [],
            "notes": "",
        }
        grounded = pipeline.ground_landscape(
            pipeline.validate_landscape(raw), "chat", allowed
        )
        feature = grounded["baseline_features"][0]
        self.assertEqual(feature["commonality"], "niche")
        self.assertEqual(feature["commonality_model"], "table_stakes")

    def test_claim_grouping_collapses_repeated_keys(self) -> None:
        """The same capability key across records becomes one claim with merged versions."""
        claims = [
            {
                "repository": "example/sample",
                "module": "chat",
                "key": "streaming",
                "name": "流式响应",
                "summary_zh": "低置信版本",
                "capability_type": "feature",
                "implementation_status": "implemented",
                "user_visible": False,
                "security_relevant": False,
                "confidence": 0.81,
                "version": "v1.0.0",
                "keywords": ["stream"],
                "source_url": "u1",
                "source_record_id": "r1",
            },
            {
                "repository": "example/sample",
                "module": "chat",
                "key": "streaming",
                "name": "流式响应（增强）",
                "summary_zh": "高置信版本",
                "capability_type": "feature",
                "implementation_status": "implemented",
                "user_visible": True,
                "security_relevant": True,
                "confidence": 0.97,
                "version": "v2.0.0",
                "keywords": ["sse"],
                "source_url": "u2",
                "source_record_id": "r2",
            },
            {
                "repository": "example/sample",
                "module": "relay_gateway",
                "key": "routing",
                "name": "路由",
                "summary_zh": "摘要",
                "capability_type": "feature",
                "implementation_status": "implemented",
                "user_visible": True,
                "security_relevant": False,
                "confidence": 0.9,
                "version": "",
                "keywords": [],
                "source_url": "u3",
                "source_record_id": "r3",
            },
        ]
        grouped = pipeline.group_claims_by_product_module(claims)
        self.assertEqual(
            sorted(grouped), [("example/sample", "chat"), ("example/sample", "relay_gateway")]
        )
        chat = grouped[("example/sample", "chat")]
        self.assertEqual(len(chat), 1)
        merged = chat[0]
        self.assertEqual(merged["occurrences"], 2)
        # Highest-confidence wording wins; boolean flags are OR-ed; versions union.
        self.assertEqual(merged["summary_zh"], "高置信版本")
        self.assertEqual(merged["confidence"], 0.97)
        self.assertTrue(merged["user_visible"])
        self.assertTrue(merged["security_relevant"])
        self.assertEqual(sorted(merged["versions"]), ["v1.0.0", "v2.0.0"])
        self.assertEqual(sorted(merged["keywords"]), ["sse", "stream"])

    def test_synthesize_end_to_end_with_fake_model(self) -> None:
        """Synthesis produces both levels, a landscape file, and markdown."""
        with tempfile.TemporaryDirectory() as repo_temporary, tempfile.TemporaryDirectory() as work_temporary:
            repo_root = Path(repo_temporary)
            workdir = Path(work_temporary)
            items_dir = workdir / "clean" / "items"
            items_dir.mkdir(parents=True)
            for index, (module, key) in enumerate(
                [("chat", "streaming"), ("chat", "history"), ("relay_gateway", "routing")]
            ):
                pipeline.atomic_write_json(
                    items_dir / f"item{index}.json",
                    {
                        "schema_version": pipeline.CLEAN_SCHEMA_VERSION,
                        "prompt_version": pipeline.PROMPT_VERSION,
                        "model": "luna-test",
                        "model_reasoning_effort": "low",
                        "repository": "example/sample",
                        "source_url": f"https://example.test/{index}",
                        "source_record_id": f"rec-{index}",
                        "needs_review": False,
                        "capabilities": [
                            {
                                "accepted_for_inventory": True,
                                "module": module,
                                "key": key,
                                "name": f"能力 {key}",
                                "summary_zh": "摘要",
                                "capability_type": "feature",
                                "implementation_status": "implemented",
                                "user_visible": True,
                                "security_relevant": False,
                                "confidence": 0.9,
                                "version": "v1.0.0",
                                "keywords": [key],
                            }
                        ],
                    },
                )

            class FakeSynth:
                def __init__(self, kind: str) -> None:
                    self.kind = kind
                    self.calls = 0

                def run(self, prompt: str, response_path: Path) -> dict:
                    self.calls += 1
                    if self.kind == "product":
                        keys = [
                            k
                            for k in ("streaming", "history", "routing")
                            if f'"key": "{k}"' in prompt
                        ]
                        return {
                            "product_features": [
                                {
                                    "key": f"pf-{keys[0]}",
                                    "name": "产品级能力",
                                    "summary_zh": "摘要",
                                    "capability_scope": "core",
                                    "user_value_zh": "价值",
                                    "maturity": "established",
                                    "user_visible": True,
                                    "security_relevant": False,
                                    "source_claim_keys": keys,
                                    "representative_versions": [],
                                }
                            ],
                            "dropped_claim_keys": [],
                            "notes": "",
                        }
                    repos = ["example/sample"]
                    pf_keys = [
                        k
                        for k in ("pf-streaming", "pf-history", "pf-routing")
                        if f'"key": "{k}"' in prompt
                    ]
                    return {
                        "baseline_features": [
                            {
                                "key": "baseline-1",
                                "name": "品类基线能力",
                                "summary_zh": "摘要",
                                "commonality": "table_stakes",
                                "capability_scope": "core",
                                "user_visible": True,
                                "security_relevant": False,
                                "product_coverage": [
                                    {
                                        "repository": repos[0],
                                        "product_feature_keys": pf_keys or ["pf-streaming"],
                                        "variant_zh": "做法",
                                    }
                                ],
                            }
                        ],
                        "unmatched_product_feature_keys": [],
                        "notes": "",
                    }

            synths: dict[str, FakeSynth] = {}

            def factory(schema_name: str) -> FakeSynth:
                kind = "product" if "product_feature" in schema_name else "landscape"
                synths.setdefault(kind, FakeSynth(kind))
                return synths[kind]

            args = argparse.Namespace(
                repo_root=repo_root,
                workdir=workdir,
                model="luna-test",
                model_reasoning_effort="low",
                codex_bin="codex",
                model_timeout=10,
                synthesis_workers=1,
                claims_per_batch=100,
                module=None,
                refresh=False,
            )
            landscape = pipeline.synthesize_command(args, synthesizer_factory=factory)

            self.assertEqual(landscape["coverage"]["products"], 1)
            self.assertEqual(landscape["coverage"]["modules"], 2)
            self.assertEqual(landscape["schema_version"], pipeline.SYNTHESIS_SCHEMA_VERSION)
            self.assertFalse(landscape["evidence_boundary"]["oblivious_implementation_proof"])
            self.assertFalse(landscape["evidence_boundary"]["current_upstream_head_verified"])
            self.assertTrue((workdir / "synthesis" / "landscape.json").exists())
            markdown = (workdir / "synthesis" / "LANDSCAPE.md").read_text(encoding="utf-8")
            self.assertIn("品类基线能力", markdown)
            self.assertIn("Evidence boundary", markdown)
            self.assertEqual(len(list((workdir / "synthesis" / "products").glob("*.json"))), 2)
            self.assertEqual(len(list((workdir / "synthesis" / "landscape").glob("*.json"))), 2)

            # Second run reuses checkpoints and makes no new model calls.
            before = synths["product"].calls + synths["landscape"].calls
            pipeline.synthesize_command(args, synthesizer_factory=factory)
            self.assertEqual(synths["product"].calls + synths["landscape"].calls, before)

    def test_redact_error_keeps_diagnostic_tail(self) -> None:
        """The real failure is at the end of codex stderr, so the tail must survive."""
        banner = "OpenAI Codex v0.147.0\n" + ("echoed prompt line\n" * 400)
        real_error = "stream disconnected before completion: upstream 429 rate limited"
        redacted = pipeline.redact_error(banner + real_error)
        self.assertLessEqual(len(redacted), 1200)
        # Tail preserved: this is the only part that identifies the failure.
        self.assertIn(real_error, redacted)
        # A short head is kept so the failing component is still identifiable.
        self.assertTrue(redacted.startswith("OpenAI Codex"))
        self.assertIn("<elided>", redacted)

        short = "boom"
        self.assertEqual(pipeline.redact_error(short), "boom")
        self.assertNotIn("<elided>", pipeline.redact_error(short))

        secret = "x" * 2000 + " api_key=SUPERSECRETVALUE trailing failure"
        cleaned = pipeline.redact_error(secret)
        self.assertNotIn("SUPERSECRETVALUE", cleaned)
        self.assertIn("trailing failure", cleaned)

    def test_clean_workers_must_be_positive(self) -> None:
        """The CLI exposes --clean-workers and rejects a non-positive worker count."""
        parser = pipeline.build_parser()
        parsed = parser.parse_args(
            ["clean", "--workdir", "/tmp/does-not-matter", "--clean-workers", "8"]
        )
        self.assertEqual(parsed.clean_workers, 8)
        self.assertEqual(
            parser.parse_args(["clean", "--workdir", "/tmp/does-not-matter"]).clean_workers, 1
        )
        for invalid in ("0", "-4"):
            with patch("sys.stderr"):
                with self.assertRaises(SystemExit):
                    pipeline.main(
                        ["clean", "--workdir", "/tmp/does-not-matter", "--clean-workers", invalid]
                    )


if __name__ == "__main__":
    unittest.main()
