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


if __name__ == "__main__":
    unittest.main()
