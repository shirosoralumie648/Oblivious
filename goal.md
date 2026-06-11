目标：按照以下设计文档完成项目实现。

参考文档：
- docs/superpowers/specs/2026-06-04-complete-fusion-design.md
- docs/superpowers/specs/2026-06-04-complete-fusion-design-part2.md
- docs/superpowers/specs/2026-06-04-complete-fusion-design-part3.md
- docs/superpowers/specs/2026-06-04-functional-logic-details.md

执行原则：
1. 先阅读并提炼四个设计文档，建立 implementation plan。
2. 不要一次性乱改全仓库，按模块拆分。
3. 能并行的功能可以创建 agent 并行推进，但必须保证接口契约一致。
4. 每完成一个可验证步骤：
   - 运行相关测试；
   - 更新必要文档；
   - git diff 自查；
   - commit；
   - push。
5. 禁止无测试直接 push。

完成标准：
- 核心功能按设计文档落地；
- 测试通过；
- 仓库状态干净；
- 已按阶段提交并 push。