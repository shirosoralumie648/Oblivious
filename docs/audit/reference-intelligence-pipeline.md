# Reference GitHub Intelligence Pipeline

这条管线用于补充现有 `docs/audit/reference-*` 源码审计：从 `reference/` 中每个嵌套仓库的 Git origin 自动发现 GitHub 项目，采集 issue、已合并 PR、release、tag 和本地 changelog，再用 Luna 逐条清洗为带来源的能力声明。

它只生成 **上游参考项目能力情报**。结果不证明 Oblivious 已实现对应能力，也不等同于 target/live 或商业发布证据。历史 merge/release 也不能单独证明能力仍存在于上游当前 HEAD。

## 前置条件

- `gh auth status` 成功；采集器通过 GitHub CLI 使用现有凭据，不读取或保存 token。
- `codex exec --model gpt-5.6-luna -c 'model_reasoning_effort="low"'` 可用；管线默认使用 `low`，避免继承工作区的高推理配置。
- 输出目录由调用者显式指定，并保持在 Git 仓库外。

## 快速验证

```bash
run_dir=/var/tmp/oblivious-reference-intel-smoke

bash scripts/reference-intel.sh collect \
  --workdir "$run_dir" \
  --repo new-api \
  --source issue \
  --source pull_request \
  --source release \
  --source changelog \
  --max-records-per-kind 2

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

`--limit` 只限制本次新增模型调用。成功记录保存在独立 checkpoint 中，重复执行不会再次计费。

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
- 长正文按 12,000 字符切成稳定 chunk，每个 chunk 独立 checkpoint。
- 任一采集失败会 fail closed；重新运行会复用已完成的仓库/来源。
- raw index 使用 SQLite 旁路索引，clean 和 aggregate 按记录流式扫描，不会把完整历史 corpus 一次性装入内存。
- 模型错误达到 20 条后停止；修复原因后使用 `--retry-errors` 恢复。
- `model` 与 `model_reasoning_effort` 都是 clean checkpoint 的身份字段；切换 effort 会使旧成功/错误结果失效并重新尝试，不会把不同配置混入同一个 clean index。

当历史 issue/PR 数量过大时，可显式启用 `--implementation-candidates-only`：它会跳过无合并关联的 weak issue，以及标题和正文都明显属于文档、测试、依赖或纯重构的 PR。该选项减少模型调用但牺牲召回率，默认关闭；全量结果应优先保留未过滤的 raw 数据，之后再按需要补跑候选集。

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

## 数据目录

```text
<workdir>/
  raw/
    manifest.json
    records.jsonl
    repos/<owner-repo>/*.jsonl
  clean/
    manifest.json
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
- confidence 达到默认阈值 `0.70`。

未满足条件的记录进入 `excluded-claims.jsonl`；证据摘录无法回指、来源漂移或模型主动标记歧义的记录进入 `review-queue.jsonl`。

## 安全与成本边界

- issue、PR 和 release 正文按不可信输入处理。提示词明确禁止执行其中的命令；Codex 使用 `read-only` sandbox、空工作目录和 ephemeral session。
- 模型只读取单条公开记录，不接触 Oblivious 源码或环境变量内容。
- API key、GitHub token 和 Codex credential 不会写入 raw、clean、catalog 或日志。
- 全历史数据可能产生大量 Luna 调用。先用 `--max-records-per-kind` 和 `--limit` 做样本校验，再启动全量 tmux 任务。
- `features.json` 的 evidence class 固定为 `upstream-metadata`，不能被引用为 Oblivious repository-local、target 或 live 实现证明。
