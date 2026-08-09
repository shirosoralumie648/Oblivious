# Reference GitHub Intelligence Pipeline

这条管线用于补充现有 `docs/audit/reference-*` 源码审计：从 `reference/` 中每个嵌套仓库的 Git origin 自动发现 GitHub 项目，采集 issue、已合并 PR、release、tag 和本地 changelog，再用指定 Codex 模型逐条清洗为带来源的能力声明。默认模型仍是 Luna；当前跨仓库质量样本按用户指示使用 `gpt-5.4-mini`、`low` effort。

它只生成 **上游参考项目能力情报**。结果不证明 Oblivious 已实现对应能力，也不等同于 target/live 或商业发布证据。历史 merge/release 也不能单独证明能力仍存在于上游当前 HEAD。

## 前置条件

- `gh auth status` 成功；采集器通过 GitHub CLI 使用现有凭据，不读取或保存 token。
- 所选 Codex 模型可用。管线默认模型为 `gpt-5.6-luna`、默认 effort 为 `low`，也可显式传入 `gpt-5.4-mini`；模型与 effort 都会进入 checkpoint 身份。
- 输出目录由调用者显式指定，并保持在 Git 仓库外。

## 快速验证

```bash
run_dir=/var/tmp/oblivious-reference-intel-smoke

bash scripts/reference-intel.sh collect \
  --workdir "$run_dir" \
  --repo new-api \
  --source release \
  --max-records-per-kind 1

bash scripts/reference-intel.sh clean \
  --workdir "$run_dir" \
  --repo new-api \
  --clean-source release \
  --model gpt-5.6-luna \
  --model-reasoning-effort low \
  --limit 1

bash scripts/reference-intel.sh aggregate --workdir "$run_dir"
bash scripts/reference-intel.sh status --workdir "$run_dir"
```

`status` 还会显示基于 strong/medium 证据的候选 record 和预计 model unit 数，适合在付费清洗前做规模确认。

当前 status 会同时报告两种范围：`unfiltered_cleanable_*` 是 issue、merged PR、release、changelog 的未过滤模型范围；`implementation_candidate_*` 是 strong/medium 证据并经过确定性候选过滤后的范围。两者必须分开记录，candidate-only 运行不能被描述成完整召回。

`--limit` 只限制本次新增模型调用。上例的 clean scope 只有一个 release unit，因此可以通过默认的完整聚合门禁。若对多条输入使用 `--limit`，聚合会拒绝不完整 clean；只有显式 `--allow-incomplete-clean` 才能生成不可发布的诊断目录。

成功记录保存在独立 checkpoint 中，重复执行不会再次计费。schema、prompt、model、effort、source record hash 或 chunk hash 变化时，旧 checkpoint 不再视为当前结果。

## 全量运行

```bash
run_dir=/var/tmp/oblivious-reference-intel-full

bash scripts/reference-intel.sh run \
  --workdir "$run_dir" \
  --model gpt-5.6-luna \
  --model-reasoning-effort low \
  --collect-workers 1 \
  --progress-every 10
```

默认行为：

- 采集所有 30 个可识别的 GitHub 仓库，不使用手工项目清单。
- GitHub 采集默认单路执行以限制 `gh --paginate` 的峰值内存；有足够内存时可显式提高 `--collect-workers`。Luna 清洗严格单条、串行执行。
- issue、merged PR、release 和 changelog 进入 Luna；tag 被采集用于版本时间线，但默认不消耗模型调用。
- issue 的实现证据会附带 closing merged PR 的 record ID、content hash、标题、正文、URL、merge SHA 与合并时间；issue 请求文本与 PR 实现文本在提示词中保持分区。
- 长正文按 12,000 字符切成稳定 chunk，每个 chunk 独立 checkpoint。
- 任一采集失败会 fail closed；重新运行会复用已完成的仓库/来源。
- raw index 使用 SQLite 旁路索引，clean 和 aggregate 按记录流式扫描，不会把完整历史 corpus 一次性装入内存。
- 模型错误达到 20 条后停止；修复原因后使用 `--retry-errors` 恢复。
- `model` 与 `model_reasoning_effort` 都是 clean checkpoint 的身份字段；切换 effort 会使旧成功/错误结果失效并重新尝试，不会把不同配置混入同一个 clean index。

当历史 issue/PR 数量过大时，可显式启用 `--implementation-candidates-only`：它会跳过无合并关联的 weak issue，以及标题和正文都明显属于文档、测试、依赖或纯重构的 PR。该选项减少模型调用但牺牲召回率，默认关闭；如果目标是“获取所有实现功能”，不能仅凭候选集声明完整召回，必须保留未过滤 raw 并另行验证过滤器召回率或补跑未过滤 clean scope。

长时间任务应放在命名 tmux 会话中：

```bash
tmux new-session -d -s oblivious-reference-intel \
  "cd /path/to/Oblivious && PYTHONUNBUFFERED=1 bash scripts/reference-intel.sh run --workdir /var/tmp/oblivious-reference-intel-full --model gpt-5.6-luna --model-reasoning-effort low 2>&1 | tee /var/tmp/oblivious-reference-intel-full/run.log"

tmux attach -t oblivious-reference-intel
```

## 分阶段恢复

```bash
bash scripts/reference-intel.sh collect --workdir "$run_dir"
bash scripts/reference-intel.sh clean --workdir "$run_dir" --model gpt-5.6-luna --model-reasoning-effort low
bash scripts/reference-intel.sh clean --workdir "$run_dir" --model gpt-5.6-luna --model-reasoning-effort low --retry-errors
bash scripts/reference-intel.sh aggregate --workdir "$run_dir"
```

使用 `--refresh` 重新拉取 GitHub 来源。默认 collection fingerprint 包含仓库、来源、`--since`、正文上限、采样上限，以及 changelog 对应的本地 snapshot SHA。

更新到 issue→merged PR 富化契约后，旧 issue collection fingerprint 会失效。必须先重新执行 `collect`，再以 `status` 的 `clean_contract_current=true` 和最终 unit 数为准；旧 clean v1 数量只能作为历史进度，不能作为可恢复的 v2 checkpoint。

## 固定样本复现

已有 selection manifest 时，可在不改动原始全量目录的前提下重建完全相同的 record-ID 样本，并从全量 raw PR 记录补齐 issue 的关联实现上下文：

```bash
bash scripts/reference-intel.sh materialize-sample \
  --source-workdir /var/tmp/oblivious-reference-intel-full \
  --selection-manifest /path/to/sample-selection.json \
  --workdir /var/tmp/oblivious-reference-intel-sample-v3
```

目标 workdir 必须为空且位于 Git 仓库外。输出的 `sample-selection.json` 会记录原 selection 摘要、materialized raw hash、关联 PR 富化状态和最终 unit 数。

## 全量 raw v3 与当前清洗

保留的旧 full raw 目录没有被原地改写。已从本地 SQLite/JSONL 快照物化出新的目录：

```text
source: /home/shirosora/.cache/oblivious-reference-intel/full-20260809
target: /home/shirosora/.cache/oblivious-reference-intel/full-20260809-v3
```

物化结果为 190,299 条记录（changelog 25、issue 88,920、merged PR 80,137、release 9,295、tag 11,922）。12,827 个 issue 解析出 14,321 个关联 merged-PR 内容上下文并重算 issue hash；其中 12,677 个 issue 达到 strong evidence，另外 150 个仍保持 open/weak，不能仅凭 issue 状态形成 strong 声明。所有非 issue 记录的内容 hash 与源快照一致，源 manifest/index 在物化前后也保持一致。目标 `raw/manifest.json` 和 `corpus-materialization.json` 保存源/目标哈希及计数。

物化后的当前 scope（`max_prompt_chars=12,000`）是：

| 范围 | records | units |
| --- | ---: | ---: |
| 未过滤 issue/PR/release/changelog | 178,377 | 179,669 |
| evidence-eligible candidate | 90,444 | 91,273 |

tag 保留在 raw 时间线中，但默认不发送给模型。上表数值应以 `status` 的对应字段为准；不要从旧 clean v1 manifest 推断当前调用量。

在一次真实单元探针通过后，candidate-only 的 `gpt-5.4-mini`、`low`、confidence `0.80` 清洗已放入具名 tmux：

```text
session: oblivious-reference-gpt54mini-full-v3-20260809
runner: /home/shirosora/.cache/oblivious-reference-intel/full-20260809-v3/run-full-v3.sh
```

runner 严格按 `clean -> aggregate -> status` 执行，缺少 `--allow-clean-errors`；clean 有 error 或 pending 时返回非零，`aggregate` 不会运行。`clean/progress.json` 是原子 checkpoint，包含 schema/prompt/model/effort、source scope、position、调用数、错误数、每调用耗时和 ETA。当前只证明任务可恢复且正在运行，不证明全量清洗完成，也不证明已经得到所有实现功能。监控命令：

```bash
tmux attach -t oblivious-reference-gpt54mini-full-v3-20260809
bash scripts/reference-intel.sh status --workdir /home/shirosora/.cache/oblivious-reference-intel/full-20260809-v3
tail -f /home/shirosora/.cache/oblivious-reference-intel/full-20260809-v3/run.log
```

## 数据目录

```text
<workdir>/
  sample-selection.json        # materialized sample only
  corpus-materialization.json  # materialized full corpus only
  raw/
    manifest.json
    records.jsonl
    repos/<owner-repo>/*.jsonl
  clean/
    manifest.json
    progress.json
    records.jsonl
    items/<unit-hash>.json
    errors/<unit-hash>.json
    responses/<unit-hash>.json
  catalog/
    features.json
    features.jsonl
    features.md
    review-queue.jsonl
    excluded-claims.jsonl
```

Raw record 的 `content_sha256` 由稳定来源字段计算；模型不能覆盖 repository、source URL、source hash、merge SHA、tag 或 evidence level。聚合时会重新校验 clean-to-raw hash join。

## 证据门禁

| 来源 | 默认等级 | 能否直接形成实现声明 |
| --- | --- | --- |
| merged PR + merge commit SHA | strong | 可以，但 docs/test/refactor/chore 会被排除 |
| published release + tag | strong | 可以，按 release 条目拆分 |
| issue linked by closing merged PR | strong | 可以，与 PR 证据保持关联 |
| tracked changelog at local snapshot | medium | 可以，但仍标记为 metadata evidence |
| tag only | medium | 默认只用于时间线 |
| open/closed issue without merge link | weak | 不可仅凭关闭状态计入 |

一个能力只有同时满足以下条件才进入 `catalog/features.*`：

- Luna 将记录判为 `implementation_bearing`；
- claim 状态为 `implemented` 或 `fixed`；
- 来源等级为 `strong` 或 `medium`；
- `evidence_excerpt` 能在输入标题或正文中逐字定位；
- confidence 达到默认阈值 `0.80`。

模型将 unit 标记为 `needs_review` 时，其中即使通过其他确定性条件的候选 claim 也只进入 `review-queue.jsonl`，不会进入 `features.*`。其余未满足条件的 claim 进入 `excluded-claims.jsonl`，并强制写入 `accepted_for_inventory=false`。聚合还会重新校验 clean-to-raw hash、schema 和 prompt；默认拒绝任何 error 或 pending unit。

## 安全与成本边界

- issue、PR 和 release 正文按不可信输入处理。提示词明确禁止执行其中的命令；Codex 使用 `read-only` sandbox、空工作目录和 ephemeral session。
- 模型只读取单条公开记录，不接触 Oblivious 源码或环境变量内容。
- API key、GitHub token 和 Codex credential 不会写入 raw、clean、catalog 或日志。
- 全历史数据可能产生大量 Luna 调用。先用 `--max-records-per-kind` 和 `--limit` 做样本校验，再启动全量 tmux 任务。
- `features.json` 的 evidence class 固定为 `upstream-metadata`，不能被引用为 Oblivious repository-local、target 或 live 实现证明。

## 当前跨仓库样本门禁

`gpt-5.4-mini` 的 v3 27-repository、54-unit 同 record-ID 样本已完成。54/54 unit 成功，144 条模型 claim 被完整分区为 120 accepted、10 review-held、14 excluded；review unit 从首轮 22/54 降至 7/54，accepted 与 review queue 零重叠。17/17 issue 均补齐关联 merged PR 上下文，针对 issue、PR、release 的人工 spot check 未发现会推翻样本门禁的误收录。

因此当前结论是：**有界跨仓库样本质量门禁通过，仍标记为 SAMPLE_ONLY；全量 raw v3 已通过物化门禁，candidate-only 清洗正在运行。** 旧 full manifest 的 94,097-unit / 20-successful 数字属于 clean v1 历史进度，不能与当前 v2/prompt v3 checkpoint 混用。candidate-only 仍是召回边界；若要支持“所有实现功能”的无保留说法，必须在候选清洗完成后补跑未过滤范围或提供独立 recall 验证。catalog 只有在完整、无 error 的 clean 后才会生成。

完整范围、统计、摘要校验值和下一门禁见 [GPT-5.4 Mini cross-repository sample](./reference-intelligence-gpt54mini-cross-repo-sample.md)。
