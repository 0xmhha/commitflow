# Git Upstream Tracker - Architecture Document

## 1. Overview

### 1.1 Problem Statement

go-ethereum과 같은 대규모 오픈소스 프로젝트에서 fork한 프로젝트들은 upstream의 변경사항을 지속적으로 선별하여 자체 코드베이스에 적용해야 한다. 이 작업은 다음과 같은 문제를 수반한다:

- upstream 커밋을 하나씩 검토하여 적용 여부를 판단하는 데 **막대한 인적 리소스** 소비
- 변경사항의 **카테고리(보안 패치, 버그 수정, 성능 개선 등)를 수동으로 분류**해야 하는 비효율
- fork 프로젝트의 자체 수정사항과 **충돌 가능성을 사전에 파악하기 어려움**
- 어떤 upstream 커밋이 이미 적용되었는지 **추적 관리의 부재**

### 1.2 Solution

`git-upstream-tracker`는 두 가지 핵심 엔진을 제공하는 CLI 도구이다:

| 엔진 | 역할 |
|------|------|
| **Commit Analysis Engine** | git 커밋을 순차 조회하며 AI로 변경 내용을 분류/요약/저장 |
| **Upstream Sync Engine** | upstream 커밋의 fork 적용 가능 여부를 판단하고 선별 적용 |

### 1.3 Tech Stack

| 구성요소 | 기술 | 선택 이유 |
|---------|------|----------|
| Language | Go | go-ethereum 생태계와 동일 언어, 빠른 실행, 단일 바이너리 배포 |
| AI | Claude Code CLI | `--json-schema`로 구조화된 출력 보장, 별도 API 키 관리 불필요 |
| Storage | SQLite (modernc.org/sqlite) | Pure Go(CGO 불필요), 로컬 파일 기반, 대량 커밋 검색/필터링에 적합 |
| CLI Framework | cobra | Go 표준 CLI 프레임워크, 서브커맨드 지원 |
| Git | git binary (shell out) | go-git은 대형 repo에서 메모리 문제. git binary는 최적화된 pack file 처리 |

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI Layer (cobra)                        │
│  analyze | report | show | upstream init/scan/status/apply/auto │
└──────────┬──────────────────────────────────┬───────────────────┘
           │                                  │
           v                                  v
┌─────────────────────┐           ┌─────────────────────────┐
│  Analysis Engine     │           │  Upstream Sync Engine    │
│  ┌───────────────┐  │           │  ┌───────────────────┐  │
│  │ engine.go     │  │           │  │ tracker.go        │  │
│  │ classifier.go │  │           │  │ applicability.go  │  │
│  │ prompt.go     │  │           │  │ applier.go        │  │
│  └───────┬───────┘  │           │  └─────────┬─────────┘  │
└──────────┼──────────┘           └────────────┼────────────┘
           │                                   │
     ┌─────┴───────────────────────────────────┘
     │
     v
┌─────────────────────┐    ┌──────────────────┐    ┌──────────────┐
│    AI Client         │    │  Git Operations  │    │   Storage    │
│  ┌───────────────┐  │    │  ┌────────────┐  │    │  ┌────────┐  │
│  │ client.go     │  │    │  │repository  │  │    │  │ db.go  │  │
│  │ parser.go     │  │    │  │commit.go   │  │    │  │commits │  │
│  │ schemas.go    │  │    │  │diff.go     │  │    │  │syncs   │  │
│  └───────┬───────┘  │    │  └─────┬──────┘  │    │  └───┬────┘  │
└──────────┼──────────┘    └────────┼─────────┘    └──────┼───────┘
           │                        │                      │
           v                        v                      v
    Claude Code CLI            git binary              SQLite DB
```

### 2.2 Project Structure

```
git-upstream-tracker/
├── cmd/                          # CLI 커맨드 (cobra)
│   ├── root.go                   # 글로벌 플래그: --config, --verbose, --dry-run, --model
│   ├── analyze.go                # tracker analyze <repo> --from <hash> --to <hash>
│   ├── report.go                 # tracker report <repo> --category <filter> --since <date>
│   ├── show.go                   # tracker show <commit-hash>
│   └── upstream/
│       ├── init.go               # tracker upstream init --upstream <url> --fork <path>
│       ├── scan.go               # tracker upstream scan --from <hash> --to <hash>
│       ├── status.go             # tracker upstream status
│       ├── apply.go              # tracker upstream apply <commit-hash>
│       ├── skip.go               # tracker upstream skip <commit-hash> --reason <text>
│       └── auto.go               # tracker upstream auto --from <hash> --to <hash>
├── internal/
│   ├── git/                      # Git 연산 래퍼 (shell out to git)
│   │   ├── repository.go         # Repository 열기, 유효성 검증
│   │   ├── commit.go             # 커밋 메타데이터 추출
│   │   └── diff.go               # Diff 추출 + chunking 전략
│   ├── analysis/                 # 커밋 분석 엔진
│   │   ├── engine.go             # 분석 오케스트레이터 (순차 처리, 재개 지원)
│   │   ├── classifier.go         # 카테고리 분류 로직
│   │   └── prompt.go             # 프롬프트 템플릿 (text/template)
│   ├── sync/                     # Upstream 동기화 엔진
│   │   ├── tracker.go            # Upstream scan 오케스트레이터
│   │   ├── applicability.go      # Fork 컨텍스트 수집, 적용 가능성 평가
│   │   └── applier.go            # Cherry-pick 실행, 충돌 처리
│   ├── ai/                       # Claude Code CLI 통합
│   │   ├── client.go             # CLI 래퍼 (rate limit, retry, budget tracking)
│   │   ├── parser.go             # ClaudeResponse JSON 파싱
│   │   └── schemas.go            # JSON schema 정의 (structured output)
│   ├── storage/                  # SQLite 영속 계층
│   │   ├── db.go                 # DB 연결 관리 (WAL 모드, pragma 설정)
│   │   ├── migrations.go         # 스키마 마이그레이션 러너
│   │   ├── commit_store.go       # commit_analyses CRUD
│   │   └── sync_store.go         # upstream_configs + upstream_syncs CRUD
│   └── config/
│       └── config.go             # YAML 설정 + 환경변수 + CLI 플래그 병합
├── go.mod
├── go.sum
├── Makefile
└── docs/
    └── ARCHITECTURE.md           # 이 문서
```

### 2.3 Component Interfaces

각 컴포넌트는 인터페이스로 정의되어 테스트 가능성과 교체 가능성을 보장한다:

```go
// AI 분석기 인터페이스
type Analyzer interface {
    AnalyzeCommit(ctx context.Context, req CommitAnalysisRequest) (*CommitAnalysisResponse, error)
    AssessApplicability(ctx context.Context, req ApplicabilityRequest) (*ApplicabilityResponse, error)
}

// Git 저장소 인터페이스
type Repository interface {
    ListCommits(ctx context.Context, opts CommitListOpts) ([]Commit, error)
    GetCommitDiff(ctx context.Context, hash string) (*DiffResult, error)
    CherryPick(ctx context.Context, hash string) (*CherryPickResult, error)
    AbortCherryPick(ctx context.Context) error
}

// 커밋 분석 저장소 인터페이스
type CommitStore interface {
    SaveAnalysis(ctx context.Context, analysis *CommitAnalysis) error
    GetAnalysis(ctx context.Context, repoPath, hash string) (*CommitAnalysis, error)
    ListAnalyses(ctx context.Context, filter AnalysisFilter) ([]CommitAnalysis, error)
    IsAnalyzed(ctx context.Context, repoPath, hash string) (bool, error)
}

// Upstream 동기화 저장소 인터페이스
type SyncStore interface {
    SaveConfig(ctx context.Context, cfg *UpstreamConfig) error
    GetConfig(ctx context.Context, name string) (*UpstreamConfig, error)
    SaveSync(ctx context.Context, sync *UpstreamSync) error
    ListSyncs(ctx context.Context, filter SyncFilter) ([]UpstreamSync, error)
    UpdateSyncStatus(ctx context.Context, id int64, status, reason string) error
}
```

---

## 3. Claude Code CLI Integration

### 3.1 Invocation Pattern

```bash
claude -p "<prompt>" \
  --output-format json \
  --model sonnet \
  --tools "" \
  --no-session-persistence \
  --json-schema '<schema>' \
  --max-budget-usd <limit>
```

| 플래그 | 목적 |
|--------|------|
| `--output-format json` | JSON 응답 강제 |
| `--model sonnet` | 비용/품질 균형 (복잡한 경우 opus 오버라이드 가능) |
| `--tools ""` | 도구 비활성화 → 시스템 프롬프트 토큰 ~10K 절약 |
| `--no-session-persistence` | 수백 번 호출 시 세션 파일 축적 방지 |
| `--json-schema` | `structured_output` 필드에 보장된 스키마의 JSON 출력 |
| `--max-budget-usd` | 호출당 비용 안전장치 |

### 3.2 Response Structure

```go
type ClaudeResponse struct {
    Type             string          `json:"type"`
    IsError          bool            `json:"is_error"`
    DurationMS       int             `json:"duration_ms"`
    Result           string          `json:"result"`
    TotalCostUSD     float64         `json:"total_cost_usd"`
    Usage            ClaudeUsage     `json:"usage"`
    StructuredOutput json.RawMessage `json:"structured_output"`
}

type ClaudeUsage struct {
    InputTokens              int `json:"input_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
    OutputTokens             int `json:"output_tokens"`
}
```

### 3.3 Commit Analysis Schema

Claude가 각 커밋에 대해 반환하는 구조화된 출력:

```json
{
  "type": "object",
  "properties": {
    "category": {
      "type": "string",
      "enum": ["bug_fix", "feature", "refactor", "security",
               "performance", "docs", "chore", "breaking_change"]
    },
    "summary": {
      "type": "string",
      "description": "커밋의 목적과 변경 내용을 설명하는 한 문단 요약"
    },
    "impact_score": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10,
      "description": "코드베이스에 미치는 영향도 (1=사소, 10=크리티컬)"
    },
    "breaking_changes": { "type": "boolean" },
    "breaking_change_details": { "type": "string" },
    "packages_affected": {
      "type": "array",
      "items": { "type": "string" }
    },
    "security_relevant": { "type": "boolean" },
    "security_details": { "type": "string" },
    "detailed_analysis": {
      "type": "string",
      "description": "변경사항의 기술적 분석 (다중 문단)"
    }
  },
  "required": ["category", "summary", "impact_score",
               "breaking_changes", "packages_affected", "security_relevant"]
}
```

### 3.4 Applicability Assessment Schema

upstream 커밋의 fork 적용 가능 여부 판단 결과:

```json
{
  "type": "object",
  "properties": {
    "status": {
      "type": "string",
      "enum": ["applicable", "not_applicable", "needs_review", "already_applied"]
    },
    "reason": {
      "type": "string",
      "description": "판단 근거 설명"
    },
    "relevance_score": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10
    },
    "conflict_likelihood": {
      "type": "string",
      "enum": ["none", "low", "medium", "high"]
    },
    "recommended_action": {
      "type": "string",
      "description": "권장 조치 설명"
    },
    "affected_fork_packages": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["status", "reason", "relevance_score",
               "conflict_likelihood", "recommended_action"]
}
```

---

## 4. Data Model

### 4.1 SQLite Schema

```sql
-- 커밋 분석 결과
CREATE TABLE commit_analyses (
    id INTEGER PRIMARY KEY,
    repo_path TEXT NOT NULL,
    commit_hash TEXT NOT NULL,
    parent_hash TEXT,
    author TEXT,
    author_email TEXT,
    commit_date TEXT,
    message TEXT,
    files_changed TEXT,           -- JSON array of file paths
    diff_stats TEXT,              -- JSON: {"additions": N, "deletions": N, "files_count": N}
    category TEXT,                -- bug_fix|feature|refactor|security|performance|docs|chore|breaking_change
    summary TEXT,                 -- AI 생성 요약
    detailed_analysis TEXT,       -- AI 상세 분석
    impact_score INTEGER,         -- 1-10
    breaking_changes BOOLEAN DEFAULT FALSE,
    packages_affected TEXT,       -- JSON array
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_path, commit_hash)
);

-- Upstream 설정
CREATE TABLE upstream_configs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,    -- 식별 이름 (예: "geth")
    upstream_url TEXT NOT NULL,   -- upstream remote URL
    fork_path TEXT NOT NULL,      -- fork 로컬 경로
    last_synced_hash TEXT,        -- 마지막 스캔 커밋 해시
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Upstream 동기화 추적
CREATE TABLE upstream_syncs (
    id INTEGER PRIMARY KEY,
    config_id INTEGER REFERENCES upstream_configs(id),
    upstream_commit TEXT NOT NULL,
    status TEXT DEFAULT 'pending', -- pending|applicable|not_applicable|applied|skipped|conflict
    applicability_reason TEXT,     -- AI 판단 근거
    relevance_score INTEGER,       -- 1-10
    applied_at TEXT,
    applied_commit TEXT,           -- fork에 적용된 커밋 해시
    skip_reason TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(config_id, upstream_commit)
);

-- AI 호출 비용 추적
CREATE TABLE ai_call_log (
    id INTEGER PRIMARY KEY,
    call_type TEXT NOT NULL,       -- analysis|applicability
    commit_hash TEXT,
    model TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cost_usd REAL,
    duration_ms INTEGER,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- 인덱스
CREATE INDEX idx_commit_analyses_hash ON commit_analyses(repo_path, commit_hash);
CREATE INDEX idx_commit_analyses_category ON commit_analyses(repo_path, category);
CREATE INDEX idx_upstream_syncs_commit ON upstream_syncs(config_id, upstream_commit);
CREATE INDEX idx_upstream_syncs_status ON upstream_syncs(config_id, status);
```

### 4.2 SQLite Configuration

```go
// WAL 모드 + 성능 최적화 Pragma
PRAGMA journal_mode = WAL;          // 읽기-쓰기 동시성
PRAGMA busy_timeout = 5000;         // Lock 대기 5초
PRAGMA synchronous = NORMAL;        // 성능/안전성 균형
PRAGMA foreign_keys = ON;           // FK 제약조건 강제
PRAGMA cache_size = -64000;         // 64MB 캐시
PRAGMA mmap_size = 268435456;       // 256MB mmap
```

---

## 5. Core Workflows

### 5.1 Commit Analysis Workflow

`tracker analyze <repo-path> --from <hash> --to <hash>` 실행 시:

```
1. INITIALIZE
   ├── config 로드 (YAML + env + flags)
   ├── SQLite DB 열기 (없으면 생성 + 마이그레이션)
   ├── repo-path가 git repository인지 검증
   └── --from, --to를 full commit hash로 resolve

2. ENUMERATE COMMITS
   ├── git rev-list --reverse <from>..<to>
   ├── 시간순 커밋 해시 목록 획득
   └── --last N 옵션: git rev-list --reverse -N HEAD

3. FILTER ALREADY-ANALYZED
   ├── 각 해시를 DB에서 조회 (IsAnalyzed)
   ├── 미분석 커밋만 work queue에 추가
   └── 출력: "Found N commits, M already analyzed, K to analyze"

4. FOR EACH COMMIT (순차 처리, 프로그레스 바):
   │
   ├── 4a. EXTRACT METADATA
   │   └── git log -1 --format=JSON <hash>
   │       → hash, parent, author, email, date, message
   │
   ├── 4b. EXTRACT DIFF
   │   ├── git diff --stat <parent>..<hash>     (항상)
   │   ├── git diff --numstat <parent>..<hash>   (파일별 라인 수)
   │   └── Diff Chunking Strategy 적용 (섹션 6.1 참조)
   │
   ├── 4c. BUILD PROMPT
   │   └── 메타데이터 + diff → 프롬프트 템플릿 채우기
   │
   ├── 4d. DRY RUN CHECK
   │   └── --dry-run이면 프롬프트만 출력, Claude 호출 skip
   │
   ├── 4e. CALL CLAUDE
   │   ├── claude -p "<prompt>" --output-format json --json-schema ...
   │   ├── ClaudeResponse 파싱
   │   ├── is_error 체크
   │   ├── structured_output → CommitAnalysisResult 매핑
   │   └── cost/token 사용량 기록
   │
   ├── 4f. STORE RESULT
   │   └── CommitAnalysis → SQLite 저장
   │
   ├── 4g. PROGRESS UPDATE
   │   └── [K/Total] abc1234 => bug_fix (impact: 7/10) - Fix memory leak in...
   │
   └── 4h. RATE LIMIT
       └── DelayBetweenCalls 대기

5. SUMMARY
   └── 총 분석 수, 총 비용, 카테고리별 분포, 에러/스킵 커밋 출력
```

### 5.2 Upstream Sync Workflow

#### 5.2.1 Init: `tracker upstream init`

```
1. fork path가 git repo인지 검증
2. upstream remote 추가 (없으면): git remote add upstream <url>
3. upstream fetch: git fetch upstream
4. 분기점 탐지: git merge-base upstream/master HEAD
5. upstream_configs 테이블에 설정 저장
6. 출력: "Initialized. Fork diverged from upstream at <merge-base>"
```

#### 5.2.2 Scan: `tracker upstream scan`

```
1. INITIALIZE
   ├── upstream config 로드
   ├── git fetch upstream
   └── commit 범위 resolve (기본: last_synced_hash..upstream/master)

2. ENUMERATE UPSTREAM COMMITS
   ├── git rev-list --reverse <from>..<to>
   └── 이미 스캔된 커밋 필터링

3. PRE-FILTER (빠른 필터링, AI 호출 없음)
   For each upstream commit:
   ├── 변경 파일 목록 추출: git diff --name-only <parent>..<hash>
   ├── fork에 해당 파일 존재 여부 확인
   │   └── 겹치는 파일 없음 → status: not_applicable (skip AI)
   ├── 이미 적용된 커밋 탐지: git cherry / git patch-id
   │   └── 이미 적용됨 → status: already_applied (skip AI)
   └── 나머지만 AI 평가 대상

4. AI ASSESSMENT (pre-filter 통과 커밋만)
   For each commit:
   ├── commit analysis 실행 (또는 기존 결과 재사용)
   ├── fork context 수집:
   │   ├── fork 전용 디렉토리 목록
   │   ├── 변경 파일의 fork 내 존재 여부
   │   └── (선택) dry-run cherry-pick으로 충돌 미리보기
   ├── applicability 프롬프트 구성
   ├── Claude 호출 → 적용 가능성 평가
   └── upstream_syncs 테이블에 결과 저장

5. UPDATE TRACKING
   ├── last_synced_hash 갱신
   └── 요약 출력: N applicable, M not_applicable, K needs_review
```

#### 5.2.3 Apply: `tracker upstream apply <hash>`

```
1. sync 레코드 조회 (status: applicable 또는 needs_review)
2. 작업 브랜치 생성: git checkout -b upstream/<hash[:8]>
3. cherry-pick 시도: git cherry-pick <hash>
4. IF 성공:
   ├── sync 레코드 갱신: status=applied, applied_commit=<new-hash>
   └── 성공 메시지 출력
5. IF 충돌:
   ├── 충돌 파일 목록: git diff --name-only --diff-filter=U
   ├── (선택) Claude에 충돌 해결 요청
   ├── 해결 성공 → cherry-pick 완료
   └── 해결 실패 → abort, status=conflict, 수동 해결 안내 출력
```

#### 5.2.4 Auto: `tracker upstream auto`

```
1. scan 실행 (5.2.2)
2. applicable 커밋을 날짜순으로 정렬
3. 각 커밋에 대해 apply 실행 (5.2.3)
   ├── 성공 → 다음 커밋으로
   ├── 충돌 → conflict 마킹, skip, 다음 커밋으로
   └── 에러 → 로그 기록, 다음 커밋으로
4. 요약 출력: N applied, M conflicts, K skipped
```

---

## 6. Key Design Decisions

### 6.1 Diff Chunking Strategy

go-ethereum 커밋은 수만 줄의 diff를 생성할 수 있다. 3단계 전략으로 처리한다:

| Diff 크기 | 처리 방식 |
|-----------|----------|
| **< 5,000 lines** | 전체 diff를 Claude에 전송 |
| **5,000 - 20,000 lines** | `.go` 파일 우선 전송, 나머지는 파일 레벨 요약 (`--stat` 형태) |
| **> 20,000 lines** | `--stat` 출력 + 커밋 메시지만 전송. 파일 경로/이름 기반 분류 |

```go
type DiffResult struct {
    FullDiff       string      // Threshold 이하일 때만 채워짐
    StatSummary    string      // 항상 존재: git diff --stat
    FileDiffs      []FileDiff  // 개별 파일 diff
    TotalLines     int
    TotalAdditions int
    TotalDeletions int
    Truncated      bool
    TruncationNote string
}
```

`--max-diff-lines` 플래그로 임계값 조정 가능 (기본: 5000).

### 6.2 Pre-filter Strategy (비용 절감)

AI 호출 전에 빠른 필터링으로 불필요한 호출을 줄인다:

```
Upstream 커밋 100개
    │
    ├── [Pre-filter 1] 파일 겹침 확인 → 40개 제외 (fork에 해당 파일 없음)
    ├── [Pre-filter 2] git patch-id 비교 → 15개 제외 (이미 적용됨)
    │
    └── AI 호출 대상: 45개 (55% 절감)
```

### 6.3 Rate Limiting & Budget Management

```go
type ClientConfig struct {
    Model              string        // 기본: "sonnet"
    MaxBudgetPerCall   float64       // 기본: 0.50 USD
    TotalBudgetLimit   float64       // 기본: 0 (무제한)
    DelayBetweenCalls  time.Duration // 기본: 1초
    MaxRetries         int           // 기본: 3
    RetryBackoff       time.Duration // 기본: 5초
    DryRun             bool          // 프롬프트만 출력
}
```

- 호출 간 설정 가능한 딜레이 (기본 1초)
- 호출당/세션 전체 비용 한도
- `ai_call_log` 테이블에 모든 호출 비용 기록
- `--dry-run` 모드로 프롬프트 확인 후 실행

### 6.4 Graceful Shutdown & Resumability

```
SIGINT/SIGTERM 수신 시:
  1. context 취소
  2. 현재 Claude 호출 완료 대기 (진행 중인 호출은 버리지 않음)
  3. SQLite flush
  4. 부분 완료 요약 출력

재실행 시:
  → DB에서 미분석 커밋 자동 감지
  → 마지막 분석 지점부터 자동 이어하기
```

---

## 7. Error Handling

### 7.1 Error Categories

| 카테고리 | 예시 | 처리 |
|---------|------|------|
| **Retriable** | Claude 타임아웃, rate limit, git network 실패 | 최대 3회 재시도 + exponential backoff |
| **Fatal** | Claude CLI 미설치, 인증 만료, 유효하지 않은 repo | 즉시 중단, 명확한 에러 메시지 |
| **Skippable** | Diff 너무 큼, binary-only 커밋 | 해당 커밋 skip, 다음으로 진행 |

### 7.2 Edge Cases

| 시나리오 | 처리 |
|---------|------|
| Empty diff (merge commit) | chore 자동 분류, impact 1, AI 호출 skip |
| Binary files in diff | binary 파일 경로만 기록, diff 텍스트에서 제외 |
| Enormous single-file diff (generated code) | 파일 요약만 전송: "File X: +N/-M lines (generated/vendored)" |
| Claude CLI 미설치 | `exec.LookPath("claude")` 체크, 명확한 설치 안내 |
| Cherry-pick of merge commit | `git cherry-pick -m 1 <hash>` 또는 경고 후 skip |
| Fork이 force-push됨 | `git rev-list --ancestry-path` 실패 감지, re-init 안내 |
| SQLite locked | WAL 모드 + busy_timeout=5000으로 대부분 해소 |
| Unicode/non-UTF8 in diff | replacement character로 치환 후 전송 |

---

## 8. CLI Commands Reference

### 8.1 Analysis Commands

```bash
# 특정 범위의 커밋 분석
tracker analyze /path/to/repo --from abc1234 --to def5678

# 최근 N개 커밋 분석
tracker analyze /path/to/repo --last 50

# Dry-run (프롬프트만 확인, Claude 호출 없음)
tracker analyze /path/to/repo --last 10 --dry-run

# 특정 모델 지정
tracker analyze /path/to/repo --last 10 --model opus

# 단일 커밋 분석 결과 조회
tracker show abc1234

# 리포트 생성 (필터링)
tracker report /path/to/repo --category security --since 2025-01-01
tracker report /path/to/repo --category bug_fix --impact-min 7
```

### 8.2 Upstream Sync Commands

```bash
# Upstream 초기화
tracker upstream init --upstream https://github.com/ethereum/go-ethereum \
                      --fork /path/to/my-fork \
                      --name geth

# Upstream 커밋 스캔 (적용 가능 여부 평가)
tracker upstream scan --from abc1234 --to def5678
tracker upstream scan --last 100

# 동기화 현황 조회
tracker upstream status

# 특정 커밋 적용
tracker upstream apply abc1234

# 특정 커밋 건너뛰기
tracker upstream skip abc1234 --reason "consensus 관련 변경, fork 자체 구현 사용"

# 자동 모드 (scan + 자동 apply)
tracker upstream auto --from abc1234 --to def5678
```

### 8.3 Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/git-upstream-tracker/config.yaml` | 설정 파일 경로 |
| `--db-path` | `~/.local/share/git-upstream-tracker/tracker.db` | SQLite DB 경로 |
| `--verbose` | `false` | 상세 로그 출력 |
| `--dry-run` | `false` | Claude 호출 없이 프롬프트만 출력 |
| `--model` | `sonnet` | Claude 모델 선택 |
| `--max-diff-lines` | `5000` | Diff 최대 라인 수 |
| `--delay` | `1s` | AI 호출 간 딜레이 |
| `--budget` | `0` (무제한) | 세션 총 비용 한도 (USD) |

---

## 9. Implementation Phases

### Phase 1: Foundation

| 순서 | 파일 | 설명 |
|------|------|------|
| 1 | `go.mod`, `main.go` | Go 모듈 초기화, cobra 스켈레톤 |
| 2 | `internal/storage/db.go`, `migrations.go` | SQLite 연결, WAL, 마이그레이션 |
| 3 | `internal/storage/commit_store.go` | commit_analyses CRUD |
| 4 | `internal/git/*` | repository, commit, diff |
| 5 | `internal/config/config.go` | 설정 관리 |

### Phase 2: AI Integration

| 순서 | 파일 | 설명 |
|------|------|------|
| 6 | `internal/ai/client.go`, `parser.go` | Claude CLI 래퍼, retry, budget |
| 7 | `internal/ai/schemas.go` | JSON schema 정의 |
| 8 | `internal/analysis/prompt.go` | 프롬프트 템플릿 |
| 9 | `internal/analysis/engine.go` | 분석 오케스트레이터 |

### Phase 3: CLI (Analysis)

| 순서 | 파일 | 설명 |
|------|------|------|
| 10 | `cmd/root.go` | 글로벌 플래그 |
| 11 | `cmd/analyze.go` | analyze 커맨드 |
| 12 | `cmd/show.go` | 단일 커밋 조회 |
| 13 | `cmd/report.go` | 리포트 생성 |

### Phase 4: Upstream Sync Engine

| 순서 | 파일 | 설명 |
|------|------|------|
| 14 | `internal/storage/sync_store.go` | upstream CRUD |
| 15 | `internal/sync/tracker.go` | scan 오케스트레이터 |
| 16 | `internal/sync/applicability.go` | 적용 가능성 평가 |
| 17 | `internal/sync/applier.go` | cherry-pick + conflict |

### Phase 5: Upstream CLI

| 순서 | 파일 | 설명 |
|------|------|------|
| 18 | `cmd/upstream/init.go` | init 커맨드 |
| 19 | `cmd/upstream/scan.go` | scan 커맨드 |
| 20 | `cmd/upstream/status.go` | status 커맨드 |
| 21 | `cmd/upstream/apply.go`, `skip.go`, `auto.go` | apply/skip/auto |

### Phase 6: Polish

| 순서 | 항목 | 설명 |
|------|------|------|
| 22 | 출력 개선 | 컬러 출력, 프로그레스 바 |
| 23 | `Makefile`, `.goreleaser.yml` | 빌드, 릴리스 자동화 |

---

## 10. Testing Strategy

| 레벨 | 대상 | 방법 |
|------|------|------|
| **Unit** | storage, git, ai, config | SQLite `:memory:`, mock interfaces |
| **Integration** | analysis engine, sync engine | 테스트용 소형 git repo (`testdata/`) |
| **E2E** | 전체 CLI 파이프라인 | go-ethereum repo에서 최근 10개 커밋 분석 |
| **Upstream Sync** | scan → apply 워크플로우 | 테스트용 fork repo 생성 후 시나리오 테스트 |
