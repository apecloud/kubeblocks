# Improve Codecov Coverage 62% → 80%

## TL;DR
> **Summary**: 系统性地为 KubeBlocks 仓库中覆盖率低于 80% 的包添加测试，从高影响包（最大未覆盖行数）开始，逐步将整体 codecov 覆盖率从 62% 提升至 80%。
> **Deliverables**: 新增/增强 ~15 个包的测试文件；修改 `.codecov.yml` 启用覆盖率门禁；覆盖率验证脚本
> **Effort**: Large
> **Parallel**: YES - 4 waves
> **Critical Path**: Task 1 (codecov.yml) → Task 2-5 (Wave 2 高影响包) → Task 6-11 (Wave 3 中影响包) → Task 12-14 (Wave 4 收尾包) → Final Verification

## Context

### Original Request
用户要求将 KubeBlocks 仓库的 codecov 覆盖率从 62% 提升到 80%。

### Interview Summary
- 目标明确：62% → 80%（+18 个百分点）
- 测试基础设施已完善：Ginkgo v2 + Gomega + envtest + golang/mock
- 324 个现有测试文件（221 Ginkgo / 135 标准 testing）
- CI 使用 `make test-fast`（`-short` flag + envtest），生成 cover.out 上传 codecov
- `.codecov.yml` 已存在但为 `informational: true`（无实际门禁）

### Metis Review (gaps addressed)
- **数据修正**: `pkg/controller/component/` 实际 5742 源码行（非 24576），~1775 未覆盖行
- **codecov.yml**: 已存在，需修改而非新建
- **multicluster 0%**: 需调查根因后再测试
- **pkg/constant/**: 排除目标——常量测试价值低（823 行，27.9% 覆盖率）
- **CI 时间**: 增量上限 +5 分钟
- **生产代码**: 禁止修改（仅 `_test.go` 文件）
- **flaky 防护**: 新测试必须连续 3 次通过

### 真实覆盖率基线数据（`go test -short -cover` 实测）

| 包 | 源码行 | 当前覆盖率 | 估计未覆盖行 | 目标覆盖率 |
|---|---|---|---|---|
| pkg/controller/multicluster/ | 1258 | **0.0%** | ~1258 | 70% |
| pkg/controller/instanceset2/ | 2227 | **19.8%** | ~1786 | 75% |
| controllers/apps/util/ | 123 | **6.0%** | ~116 | 75% |
| pkg/controller/instance/ | 2359 | **35.9%** | ~1512 | 75% |
| pkg/controller/sharding/ | 523 | **50.2%** | ~261 | 80% |
| pkg/controller/plan/ | 505 | **54.1%** | ~232 | 80% |
| pkg/controller/scheduling/ | 48 | **58.3%** | ~20 | 80% |
| pkg/kbagent/util/ | ~200 | **59.0%** | ~82 | 80% |
| pkg/controller/handler/ | 480 | **64.9%** | ~168 | 80% |
| controllers/experimental/ | ~200 | **66.7%** | ~67 | 80% |
| pkg/controller/component/ | 5742 | **69.1%** | ~1775 | 82% |
| controllers/trace/ | ~600 | **73.3%** | ~160 | 80% |
| pkg/controller/kubebuilderx/ | 996 | **73.4%** | ~265 | 80% |
| pkg/controller/model/ | 709 | **73.5%** | ~188 | 80% |
| pkg/controller/render/ | 975 | **73.5%** | ~259 | 80% |
| pkg/controller/lifecycle/ | 1018 | **74.2%** | ~263 | 80% |
| pkg/controller/instanceset/ | ~2000 | **76.9%** | ~462 | 80% |
| controllers/extensions/ | ~500 | **78.0%** | ~110 | 80% |
| pkg/operations/ | ~1000 | **79.3%** | ~207 | 82% |

**总估计未覆盖行**: ~8131 行（覆盖这些的 ~60% 即可达成 80% 总目标）

## Work Objectives

### Core Objective
将 KubeBlocks 仓库的整体 codecov 行覆盖率从 62% 提升至 80%，通过为覆盖率低于 80% 的包添加有意义的测试。

### Deliverables
1. ~15 个包的新增/增强测试文件
2. 修改后的 `.codecov.yml`（启用覆盖率门禁）
3. 每包覆盖率验证通过

### Definition of Done (verifiable conditions)
```bash
# 1. 全量测试通过
KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" make test-fast

# 2. 每个目标包达到目标覆盖率
go test -short -cover ./pkg/controller/multicluster/...  # ≥ 70%
go test -short -cover ./pkg/controller/instanceset2/...  # ≥ 75%
# ... (每个包验证)

# 3. 全量覆盖率 ≥ 80%
go test -short -coverprofile=cover.out ./pkg/... ./apis/... ./controllers/... ./cmd/...
go tool cover -func=cover.out | tail -1  # ≥ 80.0%

# 4. 无 flaky 测试（连续 3 次通过）
for i in 1 2 3; do make test-fast || exit 1; done

# 5. CI 时间增量 < 5 分钟
```

### Must Have
- 每个新测试有**有意义的断言**（禁止 `Expect(err).ToNot(HaveOccurred())` 作为唯一断言）
- 每个新测试文件遵循所在包的现有测试模式
- 每个任务完成后 `make test-fast` 全量通过
- 新测试连续运行 3 次无失败

### Must NOT Have (guardrails)
- **MUST NOT** 修改生产代码（非 `_test.go` 文件），除非有独立 PR 和明确批准
- **MUST NOT** 创建新的测试框架、fixture 工厂或 mock 生成工具——复用 `pkg/testutil/`
- **MUST NOT** 添加没有有意义断言的测试
- **MUST NOT** 测试 `zz_generated.*.go` 或其他生成代码
- **MUST NOT** 依赖测试执行顺序
- **MUST NOT** 添加 flaky 测试
- **MUST NOT** 测试 `pkg/constant/`（常量定义，测试价值低）
- **MUST NOT** 修改 mock 生成方式（复用现有 `mockgen` 模式）

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after（为已有代码补测试）+ Ginkgo/envtest for controller packages + table-driven for pure logic
- QA policy: 每个任务有 agent-executed 覆盖率验证场景 + flaky 检测场景
- Evidence: `.omo/evidence/task-{N}-{slug}.txt`（`go tool cover -func` 输出）

### 覆盖率验证命令模板
```bash
# 单包覆盖率验证
KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" \
  go test -short -coverprofile=/tmp/cover_{pkg}.out ./pkg/controller/{pkg}/...
go tool cover -func=/tmp/cover_{pkg}.out | tail -1

# flaky 检测
for i in 1 2 3; do \
  KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" \
  go test -short ./pkg/controller/{pkg}/... || exit 1; \
done

# 全量回归
make test-fast
```

## Execution Strategy

### Parallel Execution Waves

**Wave 1: 基础设施**（1 任务，独立）
- Task 1: 修改 `.codecov.yml` 启用门禁

**Wave 2: 高影响包**（4 任务，可并行）
- Task 2: `pkg/controller/multicluster/` 0% → 70%
- Task 3: `pkg/controller/instanceset2/` 19.8% → 75%
- Task 4: `pkg/controller/instance/` 35.9% → 75%
- Task 5: `pkg/controller/component/` 69.1% → 82%（最大未覆盖行数）

**Wave 3: 中影响包**（6 任务，可并行）
- Task 6: `pkg/controller/plan/` 54.1% → 80%
- Task 7: `pkg/controller/sharding/` 50.2% → 80%
- Task 8: `pkg/controller/handler/` 64.9% → 80%
- Task 9: `pkg/controller/kubebuilderx/` + `model/` + `render/` 73% → 80%
- Task 10: `pkg/controller/lifecycle/` 74.2% → 80%
- Task 11: `controllers/apps/util/` + `controllers/experimental/` + `controllers/trace/`

**Wave 4: 收尾包**（3 任务，可并行）
- Task 12: `pkg/controller/instanceset/` 76.9% → 80%
- Task 13: `controllers/extensions/` + `pkg/operations/` 78-79% → 82%
- Task 14: `pkg/controller/scheduling/` + `pkg/kbagent/util/` 58-59% → 80%

### Dependency Matrix
| Task | Blocks | Blocked By | Notes |
|------|--------|------------|-------|
| 1 | F1-F4 | - | 独立，可与 Wave 2 并行 |
| 2 | F1-F4 | - | multicluster 独立 |
| 3 | F1-F4 | - | instanceset2 独立 |
| 4 | F1-F4 | - | instance 独立 |
| 5 | F1-F4 | - | component 独立 |
| 6-11 | F1-F4 | - | Wave 3 全部可并行 |
| 12-14 | F1-F4 | - | Wave 4 全部可并行 |

### Agent Dispatch Summary
| Wave | Task Count | Categories |
|------|-----------|------------|
| 1 | 1 | quick |
| 2 | 4 | deep |
| 3 | 6 | deep |
| 4 | 3 | unspecified-high |
| Final | 4 | oracle, unspecified-high, unspecified-high, deep |

## TODOs

- [ ] 1. 修改 `.codecov.yml` 启用覆盖率门禁

  **What to do**: 修改现有 `.codecov.yml`，将 `informational: true` 改为 `false`（project status），设置 `target: 80%`，`threshold: 1%`（允许 1% 容差）。patch status 保持 `informational: true`（不阻塞 PR 上的新代码）。添加 `ignore` 列表排除 `pkg/client/`（生成代码）、`pkg/testutil/`（测试工具）、`pkg/constant/`（常量）、`zz_generated*.go`。
  
  **Must NOT do**: 不删除现有的 `flags`、`branches` 配置；不修改 `patch` status（保持 informational）

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: 单文件配置修改
  - Skills: [] - 无需特殊技能

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Config: `.codecov.yml` - 当前配置（`informational: true`, `target: auto`）
  - CI: `.github/workflows/cicd-pull-request.yml:115-131` - codecov 上传逻辑
  - CI: `.github/workflows/cicd-push.yml:140-155` - codecov 上传逻辑
  - Makefile:180 - `OUTPUT_COVERAGE` 过滤 zz_generated.deepcopy.go

  **Acceptance Criteria**:
  - [ ] `.codecov.yml` 中 project.default.informational 为 `false`
  - [ ] `.codecov.yml` 中 project.default.target 为 `80%`
  - [ ] `.codecov.yml` 中 project.default.threshold 为 `1%`
  - [ ] `.codecov.yml` 中 patch.default.informational 保持 `true`
  - [ ] `ignore` 列表包含 `pkg/client/`、`pkg/testutil/`、`pkg/constant/`
  - [ ] YAML 语法验证通过：`python3 -c "import yaml; yaml.safe_load(open('.codecov.yml'))"`

  **QA Scenarios**:
  ```
  Scenario: YAML 语法验证
    Tool: Bash
    Steps: python3 -c "import yaml; yaml.safe_load(open('.codecov.yml'))"
    Expected: 无输出（语法正确），退出码 0
    Evidence: .omo/evidence/task-1-codecov-yaml.txt

  Scenario: 配置内容验证
    Tool: Bash
    Steps: grep -c "informational: false" .codecov.yml && grep "target: 80%" .codecov.yml
    Expected: informational: false 出现 1 次，target: 80% 出现 1 次
    Evidence: .omo/evidence/task-1-codecov-yaml.txt
  ```

  **Commit**: YES | Message: `chore(codecov): enable 80% coverage gate for project status` | Files: `.codecov.yml`

- [ ] 2. pkg/controller/multicluster/ 覆盖率 0% → 70%

  **What to do**: 为 `pkg/controller/multicluster/` 包添加测试。该包有 9 个源文件（1258 行），0 个测试文件，0% 覆盖率。包含 multi-cluster client wrapper、placement、manager 等。先调查 0% 根因——是否依赖外部集群连接？如果是，用 mock client 测试。创建 `suite_test.go`（envtest setup）和针对 `client.go`、`manager.go`、`utils.go`、`placement.go`、`options.go` 的测试文件。使用标准 `testing` + table-driven 模式（该包是纯逻辑工具包，非 reconciler，不需要 Ginkgo）。用 `gomock` mock `controller-runtime` client（参考 `pkg/testutil/k8s/mocks/`）。
  
  **Must NOT do**: 不修改 multicluster 包的生产代码；不创建新的 mock 生成工具

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 0% → 70% 需要大量测试编写，需先调查根因
  - Skills: [`golang`] - Go 测试模式
  - Omitted: [`tdd`] - 补测试非 TDD

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/multicluster/client.go` (405 行) - 主要 client wrapper
  - Source: `pkg/controller/multicluster/manager.go` - multi-cluster manager
  - Source: `pkg/controller/multicluster/placement.go` - placement logic
  - Source: `pkg/controller/multicluster/utils.go` - utilities
  - Source: `pkg/controller/multicluster/options.go` - options
  - Source: `pkg/controller/multicluster/types.go` - type definitions
  - Source: `pkg/controller/multicluster/error.go` - error types
  - Source: `pkg/controller/multicluster/setup.go` - setup logic
  - Source: `pkg/controller/multicluster/client_unavailable.go` - unavailable client
  - Mock pattern: `pkg/testutil/k8s/mocks/generate.go` - mockgen 生成模式
  - Mock pattern: `pkg/testutil/k8s/mocks/k8sclient_mocks.go` - 生成的 mock
  - Test pattern: `pkg/controller/graph/dag_test.go` - 纯逻辑 table-driven 测试示例

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/multicluster/...` 覆盖率 ≥ 70.0%
  - [ ] 连续 3 次 `go test -short ./pkg/controller/multicluster/...` 全部通过
  - [ ] `make test-fast` 全量通过
  - [ ] 新测试文件包含有意义的断言（非仅 `Expect(err).ToNot(HaveOccurred())`）

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_mc.out ./pkg/controller/multicluster/... && go tool cover -func=/tmp/cover_mc.out | tail -1
    Expected: coverage ≥ 70.0% of statements
    Evidence: .omo/evidence/task-2-multicluster.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/multicluster/... || exit 1; done
    Expected: 3 次运行全部 PASS，退出码 0
    Evidence: .omo/evidence/task-2-multicluster-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过，退出码 0
    Evidence: .omo/evidence/task-2-multicluster-regression.txt
  ```

  **Commit**: YES | Message: `test(multicluster): add tests for multi-cluster client and manager, 0% → 70%` | Files: `pkg/controller/multicluster/*_test.go`

- [ ] 3. pkg/controller/instanceset2/ 覆盖率 19.8% → 75%

  **What to do**: 为 `pkg/controller/instanceset2/` 包增强测试。该包有 2227 源码行，19.8% 覆盖率，已有 4 个测试文件（706 行测试代码）。这是 instanceset 的 v2 版本 reconciler 逻辑。需要大量增加测试覆盖。先运行 `go tool cover -func` 识别未覆盖的函数，然后为关键未覆盖函数添加测试。使用 Ginkgo/envtest 模式（参考 `pkg/controller/instanceset/` 的测试模式）。
  
  **Must NOT do**: 不修改生产代码；不复制 instanceset v1 的测试（v2 API 不同）

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 大量测试编写，55% 覆盖率缺口
  - Skills: [`golang`] - Go 测试模式

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source dir: `pkg/controller/instanceset2/` - 2227 行源码
  - Existing tests: `pkg/controller/instanceset2/*_test.go` - 4 个现有测试文件
  - Test pattern: `pkg/controller/instanceset/` - v1 测试模式参考
  - envtest pattern: `controllers/apps/suite_test.go` - envtest setup 模式
  - Ginkgo pattern: `controllers/workloads/role_event_handler_test.go` - Ginkgo 测试模式

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/instanceset2/...` 覆盖率 ≥ 75.0%
  - [ ] 连续 3 次 `go test -short ./pkg/controller/instanceset2/...` 全部通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_is2.out ./pkg/controller/instanceset2/... && go tool cover -func=/tmp/cover_is2.out | tail -1
    Expected: coverage ≥ 75.0% of statements
    Evidence: .omo/evidence/task-3-instanceset2.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/instanceset2/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-3-instanceset2-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-3-instanceset2-regression.txt
  ```

  **Commit**: YES | Message: `test(instanceset2): expand tests to reach 75% coverage` | Files: `pkg/controller/instanceset2/*_test.go`

- [ ] 4. pkg/controller/instance/ 覆盖率 35.9% → 75%

  **What to do**: 为 `pkg/controller/instance/` 包增强测试。该包有 2359 源码行，35.9% 覆盖率，已有测试文件（838 行测试代码）。这是 instance 管理逻辑。先运行 `go tool cover -func` 识别未覆盖函数，重点覆盖 instance 创建、更新、删除路径。使用 Ginkgo/envtest 模式。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 大量测试编写，39% 覆盖率缺口
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source dir: `pkg/controller/instance/` - 2359 行源码
  - Existing tests: `pkg/controller/instance/*_test.go`
  - Test pattern: `pkg/controller/instanceset/` - 相邻包测试模式
  - envtest pattern: `controllers/apps/suite_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/instance/...` 覆盖率 ≥ 75.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_inst.out ./pkg/controller/instance/... && go tool cover -func=/tmp/cover_inst.out | tail -1
    Expected: coverage ≥ 75.0% of statements
    Evidence: .omo/evidence/task-4-instance.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/instance/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-4-instance-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-4-instance-regression.txt
  ```

  **Commit**: YES | Message: `test(instance): expand tests to reach 75% coverage` | Files: `pkg/controller/instance/*_test.go`

- [ ] 5. pkg/controller/component/ 覆盖率 69.1% → 82%

  **What to do**: 为 `pkg/controller/component/` 包增强测试。该包有 5742 源码行（最大包），69.1% 覆盖率，已有 11 个测试文件（7065 行测试代码）。~1775 未覆盖行——这是全仓库最大的未覆盖行数。重点覆盖大文件中的未覆盖函数：`vars.go` (1637行)、`kbagent.go` (624行)、`available.go` (580行)、`synthesize_component.go` (567行)、`workload.go` (422行)、`service_reference.go` (417行)、`component_version.go` (407行)。先运行 `go tool cover -func` 精确定位未覆盖函数。
  
  **Must NOT do**: 不修改生产代码；不为纯 getter/setter 写无意义测试

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 最大包，~1775 未覆盖行，需要系统性覆盖
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/component/vars.go` (1637 行) - 变量处理逻辑
  - Source: `pkg/controller/component/kbagent.go` (624 行) - kbagent 集成
  - Source: `pkg/controller/component/available.go` (580 行) - 可用性检查
  - Source: `pkg/controller/component/synthesize_component.go` (567 行) - 组件合成
  - Source: `pkg/controller/component/workload.go` (422 行) - 工作负载
  - Source: `pkg/controller/component/service_reference.go` (417 行) - 服务引用
  - Source: `pkg/controller/component/component_version.go` (407 行) - 组件版本
  - Existing tests: `pkg/controller/component/*_test.go` - 11 个测试文件
  - Mock pattern: `pkg/controller/component/mock_reader.go` - 现有 mock

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/component/...` 覆盖率 ≥ 82.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_comp.out ./pkg/controller/component/... && go tool cover -func=/tmp/cover_comp.out | tail -1
    Expected: coverage ≥ 82.0% of statements
    Evidence: .omo/evidence/task-5-component.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/component/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-5-component-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-5-component-regression.txt
  ```

  **Commit**: YES | Message: `test(component): expand tests for vars, kbagent, available, synthesize, 69% → 82%` | Files: `pkg/controller/component/*_test.go`

- [ ] 6. pkg/controller/plan/ 覆盖率 54.1% → 80%

  **What to do**: 为 `pkg/controller/plan/` 包增强测试。该包有 505 源码行，54.1% 覆盖率。重点覆盖 `restore.go` (416 行) 中的未覆盖路径——backup restore 逻辑的各个分支。使用 table-driven 测试模式（纯逻辑，可能不需要 envtest）。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 26% 覆盖率缺口
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/plan/restore.go` (416 行) - restore 逻辑
  - Source: `pkg/controller/plan/` - 其他文件
  - Existing tests: `pkg/controller/plan/*_test.go`
  - Test pattern: table-driven（参考 `pkg/controller/graph/dag_test.go`）

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/plan/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_plan.out ./pkg/controller/plan/... && go tool cover -func=/tmp/cover_plan.out | tail -1
    Expected: coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-6-plan.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/plan/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-6-plan-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-6-plan-regression.txt
  ```

  **Commit**: YES | Message: `test(plan): expand restore tests to reach 80% coverage` | Files: `pkg/controller/plan/*_test.go`

- [ ] 7. pkg/controller/sharding/ 覆盖率 50.2% → 80%

  **What to do**: 为 `pkg/controller/sharding/` 包增强测试。523 源码行，50.2% 覆盖率。sharding 逻辑的分片、调度路径。使用 Ginkgo 或 table-driven 模式。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 30% 覆盖率缺口
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source dir: `pkg/controller/sharding/` - 523 行
  - Existing tests: `pkg/controller/sharding/*_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/sharding/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_shard.out ./pkg/controller/sharding/... && go tool cover -func=/tmp/cover_shard.out | tail -1
    Expected: coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-7-sharding.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/sharding/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-7-sharding-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-7-sharding-regression.txt
  ```

  **Commit**: YES | Message: `test(sharding): expand tests to reach 80% coverage` | Files: `pkg/controller/sharding/*_test.go`

- [ ] 8. pkg/controller/handler/ 覆盖率 64.9% → 80%

  **What to do**: 为 `pkg/controller/handler/` 包增强测试。480 源码行，64.9% 覆盖率。event handler 逻辑。使用 Ginkgo 模式（参考现有 `handler_builder_test.go`）。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 15% 覆盖率缺口
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source dir: `pkg/controller/handler/` - 480 行
  - Existing tests: `pkg/controller/handler/handler_builder_test.go`、`pkg/controller/handler/suite_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/handler/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_handler.out ./pkg/controller/handler/... && go tool cover -func=/tmp/cover_handler.out | tail -1
    Expected: coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-8-handler.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/handler/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-8-handler-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-8-handler-regression.txt
  ```

  **Commit**: YES | Message: `test(handler): expand event handler tests to reach 80% coverage` | Files: `pkg/controller/handler/*_test.go`

- [ ] 9. pkg/controller/kubebuilderx/ + model/ + render/ 覆盖率 73% → 80%

  **What to do**: 为三个相邻中等覆盖率包增强测试。kubebuilderx (996行, 73.4%)、model (709行, 73.5%)、render (975行, 73.5%)。每个包只需提升 ~7%——覆盖少量未覆盖函数即可。先运行 `go tool cover -func` 识别低覆盖函数。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 三个包需要协调，但每个包增量小
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/kubebuilderx/` - 996 行
  - Source: `pkg/controller/model/` - 709 行
  - Source: `pkg/controller/render/` - 975 行
  - Existing tests: 各包的 `*_test.go` 文件

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/kubebuilderx/...` 覆盖率 ≥ 80.0%
  - [ ] `go test -short -cover ./pkg/controller/model/...` 覆盖率 ≥ 80.0%
  - [ ] `go test -short -cover ./pkg/controller/render/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 三包覆盖率达标
    Tool: Bash
    Steps: for pkg in kubebuilderx model render; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_${pkg}.out ./pkg/controller/${pkg}/... && go tool cover -func=/tmp/cover_${pkg}.out | tail -1; done
    Expected: 每个包 coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-9-kubebuilderx-model-render.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for pkg in kubebuilderx model render; do for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/${pkg}/... || exit 1; done; done
    Expected: 9 次运行全部 PASS
    Evidence: .omo/evidence/task-9-kubebuilderx-model-render-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-9-kubebuilderx-model-render-regression.txt
  ```

  **Commit**: YES | Message: `test(kubebuilderx,model,render): push coverage to 80%` | Files: `pkg/controller/{kubebuilderx,model,render}/*_test.go`

- [ ] 10. pkg/controller/lifecycle/ 覆盖率 74.2% → 80%

  **What to do**: 为 `pkg/controller/lifecycle/` 包增强测试。1018 源码行，74.2% 覆盖率。重点覆盖 `kbagent.go` (538行) 的未覆盖路径。只需提升 ~6%。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 小增量
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/lifecycle/kbagent.go` (538 行) - kbagent 生命周期
  - Existing tests: `pkg/controller/lifecycle/*_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/lifecycle/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_lc.out ./pkg/controller/lifecycle/... && go tool cover -func=/tmp/cover_lc.out | tail -1
    Expected: coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-10-lifecycle.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/lifecycle/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-10-lifecycle-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-10-lifecycle-regression.txt
  ```

  **Commit**: YES | Message: `test(lifecycle): expand kbagent tests to reach 80% coverage` | Files: `pkg/controller/lifecycle/*_test.go`

- [ ] 11. controllers/apps/util/ + controllers/experimental/ + controllers/trace/ 覆盖率提升

  **What to do**: 为三个小型低覆盖率 controller 包增强测试。`controllers/apps/util/` (123行, 6.0%)、`controllers/experimental/` (~200行, 66.7%)、`controllers/trace/` (~600行, 73.3%)。apps/util 只需覆盖 mock_reader.go 和工具函数。experimental 和 trace 需要覆盖 reconciler 路径。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: 三个包需要协调
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `controllers/apps/util/` - 123 行，6.0% 覆盖率
  - Source: `controllers/experimental/` - ~200 行，66.7%
  - Source: `controllers/trace/` - ~600 行，73.3%
  - Test pattern: `controllers/apps/suite_test.go` - envtest setup
  - Mock: `controllers/apps/util/mock_reader.go` - 现有 mock
  - Mock: `controllers/trace/mock_client.go`、`controllers/trace/mock_event_recorder.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./controllers/apps/util/...` 覆盖率 ≥ 75.0%
  - [ ] `go test -short -cover ./controllers/experimental/...` 覆盖率 ≥ 80.0%
  - [ ] `go test -short -cover ./controllers/trace/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 三包覆盖率达标
    Tool: Bash
    Steps: for pkg in controllers/apps/util controllers/experimental controllers/trace; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_${pkg//\//_}.out ./${pkg}/... && go tool cover -func=/tmp/cover_${pkg//\//_}.out | tail -1; done
    Expected: apps/util ≥ 75%, experimental ≥ 80%, trace ≥ 80%
    Evidence: .omo/evidence/task-11-controllers-misc.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for pkg in controllers/apps/util controllers/experimental controllers/trace; do for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./${pkg}/... || exit 1; done; done
    Expected: 9 次运行全部 PASS
    Evidence: .omo/evidence/task-11-controllers-misc-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-11-controllers-misc-regression.txt
  ```

  **Commit**: YES | Message: `test(controllers): expand util, experimental, trace tests` | Files: `controllers/{apps/util,experimental,trace}/*_test.go`

- [ ] 12. pkg/controller/instanceset/ 覆盖率 76.9% → 80%

  **What to do**: 为 `pkg/controller/instanceset/` 包增强测试。~2000 源码行，76.9% 覆盖率。只需提升 ~3%。重点覆盖 `instance_util.go` (681行)、`reconciler_status.go` (542行)、`in_place_update_util.go` (436行) 的未覆盖分支。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: 小增量，快速完成
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/instanceset/instance_util.go` (681 行)
  - Source: `pkg/controller/instanceset/reconciler_status.go` (542 行)
  - Source: `pkg/controller/instanceset/in_place_update_util.go` (436 行)
  - Existing tests: `pkg/controller/instanceset/*_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/instanceset/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 覆盖率达标
    Tool: Bash
    Steps: KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_is.out ./pkg/controller/instanceset/... && go tool cover -func=/tmp/cover_is.out | tail -1
    Expected: coverage ≥ 80.0% of statements
    Evidence: .omo/evidence/task-12-instanceset.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./pkg/controller/instanceset/... || exit 1; done
    Expected: 3 次运行全部 PASS
    Evidence: .omo/evidence/task-12-instanceset-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-12-instanceset-regression.txt
  ```

  **Commit**: YES | Message: `test(instanceset): push coverage to 80%` | Files: `pkg/controller/instanceset/*_test.go`

- [ ] 13. controllers/extensions/ + pkg/operations/ 覆盖率 78-79% → 82%

  **What to do**: 为两个接近目标的包增强测试。`controllers/extensions/` (~500行, 78.0%)、`pkg/operations/` (~1000行, 79.3%)。只需提升 ~3%。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: 小增量
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `controllers/extensions/` - ~500 行
  - Source: `pkg/operations/` - ~1000 行
  - Existing tests: 各包的 `*_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./controllers/extensions/...` 覆盖率 ≥ 82.0%
  - [ ] `go test -short -cover ./pkg/operations/...` 覆盖率 ≥ 82.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 两包覆盖率达标
    Tool: Bash
    Steps: for pkg in controllers/extensions pkg/operations; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_${pkg//\//_}.out ./${pkg}/... && go tool cover -func=/tmp/cover_${pkg//\//_}.out | tail -1; done
    Expected: 每个包 ≥ 82.0%
    Evidence: .omo/evidence/task-13-extensions-operations.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for pkg in controllers/extensions pkg/operations; do for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./${pkg}/... || exit 1; done; done
    Expected: 6 次运行全部 PASS
    Evidence: .omo/evidence/task-13-extensions-operations-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-13-extensions-operations-regression.txt
  ```

  **Commit**: YES | Message: `test(extensions,operations): push coverage to 82%` | Files: `controllers/extensions/*_test.go`, `pkg/operations/*_test.go`

- [ ] 14. pkg/controller/scheduling/ + pkg/kbagent/util/ 覆盖率 58-59% → 80%

  **What to do**: 为两个小型低覆盖率包增强测试。`pkg/controller/scheduling/` (48行, 58.3%)、`pkg/kbagent/util/` (~200行, 59.0%)。scheduling 包非常小（48行），快速完成。kbagent/util 需要覆盖工具函数。
  
  **Must NOT do**: 不修改生产代码

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: 小包，快速完成
  - Skills: [`golang`]

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: F1-F4 | Blocked By: none

  **References**:
  - Source: `pkg/controller/scheduling/` - 48 行
  - Source: `pkg/kbagent/util/` - ~200 行
  - Existing tests: 各包的 `*_test.go`

  **Acceptance Criteria**:
  - [ ] `go test -short -cover ./pkg/controller/scheduling/...` 覆盖率 ≥ 80.0%
  - [ ] `go test -short -cover ./pkg/kbagent/util/...` 覆盖率 ≥ 80.0%
  - [ ] 连续 3 次通过
  - [ ] `make test-fast` 全量通过

  **QA Scenarios**:
  ```
  Scenario: 两包覆盖率达标
    Tool: Bash
    Steps: for pkg in pkg/controller/scheduling pkg/kbagent/util; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short -coverprofile=/tmp/cover_${pkg//\//_}.out ./${pkg}/... && go tool cover -func=/tmp/cover_${pkg//\//_}.out | tail -1; done
    Expected: 每个包 ≥ 80.0%
    Evidence: .omo/evidence/task-14-scheduling-kbagent.txt

  Scenario: flaky 检测
    Tool: Bash
    Steps: for pkg in pkg/controller/scheduling pkg/kbagent/util; do for i in 1 2 3; do KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" go test -short ./${pkg}/... || exit 1; done; done
    Expected: 6 次运行全部 PASS
    Evidence: .omo/evidence/task-14-scheduling-kbagent-flaky.txt

  Scenario: 全量回归
    Tool: Bash
    Steps: make test-fast
    Expected: 全部通过
    Evidence: .omo/evidence/task-14-scheduling-kbagent-regression.txt
  ```

  **Commit**: YES | Message: `test(scheduling,kbagent-util): push coverage to 80%` | Files: `pkg/controller/scheduling/*_test.go`, `pkg/kbagent/util/*_test.go`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
  - 验证所有 14 个任务的覆盖率目标是否达成
  - 验证 `.codecov.yml` 配置是否正确
  - 验证全量覆盖率 ≥ 80%
- [ ] F2. Code Quality Review — unspecified-high
  - 检查新测试是否有有意义断言（非仅 `Expect(err).ToNot(HaveOccurred())`）
  - 检查是否遵循包内现有测试模式
  - 检查是否有 flaky 测试（3 次运行结果）
- [ ] F3. Real Manual QA — unspecified-high
  - 运行 `make test-fast` 全量回归
  - 运行 `go test -short -coverprofile=cover.out ./pkg/... ./apis/... ./controllers/... ./cmd/...` 验证总覆盖率
  - 验证 `go tool cover -func=cover.out | tail -1` 输出 ≥ 80.0%
  - 测量 `time make test-fast` 确认 CI 时间增量 < 5 分钟
- [ ] F4. Scope Fidelity Check — deep
  - 确认未修改任何生产代码（`git diff --name-only | grep -v "_test.go" | grep -v ".codecov.yml"` 应为空）
  - 确认未创建新测试框架/工具
  - 确认 `pkg/constant/` 未被测试
  - 确认所有提交信息符合 conventional commits 格式

## Commit Strategy
- 每个任务独立提交，使用 conventional commits 格式
- 提交信息模板: `test({scope}): {description}`
- scope 使用包名（如 `multicluster`, `instanceset2`, `component`）
- Task 1 使用: `chore(codecov): enable 80% coverage gate for project status`

## Success Criteria
1. **全量覆盖率 ≥ 80%**: `go test -short -coverprofile=cover.out ./pkg/... ./apis/... ./controllers/... ./cmd/... && go tool cover -func=cover.out | tail -1` 输出 ≥ 80.0%
2. **每包达标**: 所有目标包达到各自的覆盖率目标
3. **全量回归通过**: `make test-fast` 退出码 0
4. **无 flaky**: 连续 3 次 `make test-fast` 全部通过
5. **无生产代码修改**: `git diff --name-only | grep -v "_test.go" | grep -v ".codecov.yml"` 输出为空
6. **CI 时间增量 < 5 分钟**: `time make test-fast` 前后对比
7. **codecov 门禁生效**: `.codecov.yml` 中 project status `informational: false`, `target: 80%`
