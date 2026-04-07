# Architecture Diagrams

Visual architecture reference for commitflow. All diagrams use [Mermaid](https://mermaid.js.org/) syntax.

---

## System Overview

```mermaid
graph TB
    subgraph CLI["CLI Layer (cobra)"]
        analyze["commitflow analyze"]
        show["commitflow show"]
        report["commitflow report"]
        upstream_init["upstream init"]
        upstream_scan["upstream scan"]
        upstream_apply["upstream apply"]
        upstream_auto["upstream auto"]
        upstream_status["upstream status"]
        upstream_skip["upstream skip"]
    end

    subgraph Engines["Core Engines"]
        AE["Analysis Engine<br/><i>engine.go, classifier.go, prompt.go</i>"]
        SE["Sync Engine<br/><i>tracker.go, applicability.go, applier.go</i>"]
    end

    subgraph Infra["Infrastructure"]
        AI["AI Client<br/><i>client.go, parser.go, schemas.go</i>"]
        GIT["Git Operations<br/><i>repository.go, commit.go, diff.go</i>"]
        DB["Storage<br/><i>db.go, commit_store.go, sync_store.go</i>"]
        CFG["Config<br/><i>config.go</i>"]
        VAL["Validate<br/><i>git.go, sanitize.go</i>"]
    end

    subgraph External["External Systems"]
        Claude["Claude Code CLI"]
        GitBin["git binary"]
        SQLite["SQLite DB"]
    end

    analyze --> AE
    show --> DB
    report --> DB
    upstream_init --> GIT
    upstream_init --> DB
    upstream_scan --> SE
    upstream_apply --> SE
    upstream_auto --> SE
    upstream_status --> DB
    upstream_skip --> DB

    AE --> AI
    AE --> GIT
    AE --> DB
    SE --> AI
    SE --> GIT
    SE --> DB

    GIT --> VAL
    AI --> Claude
    GIT --> GitBin
    DB --> SQLite

    CLI --> CFG
```

---

## Commit Analysis Pipeline

```mermaid
flowchart TD
    Start([commitflow analyze repo --last N]) --> LoadConfig[Load Config<br/>YAML + env + flags]
    LoadConfig --> OpenDB[Open SQLite DB<br/>+ Run Migrations]
    OpenDB --> ListCommits["git rev-list --reverse -N HEAD"]
    ListCommits --> Partition{For each hash:<br/>IsAnalyzed?}

    Partition -->|Already done| Skip[Skip]
    Partition -->|Not analyzed| GetMeta["git log -1 --format=... hash"]

    GetMeta --> GetDiff["git diff --numstat parent..hash"]
    GetDiff --> Chunking{Total lines?}

    Chunking -->|"< 5,000"| FullDiff[Include full diff]
    Chunking -->|"5K - 20K"| GoOnly[".go files only<br/>+ stat summary"]
    Chunking -->|"> 20,000"| StatOnly[Stat summary only]

    FullDiff --> BuildPrompt[Build Analysis Prompt<br/>text/template]
    GoOnly --> BuildPrompt
    StatOnly --> BuildPrompt

    BuildPrompt --> DryRun{--dry-run?}
    DryRun -->|Yes| LogSkip[Log & Skip]
    DryRun -->|No| CallClaude["claude -p stdin<br/>--json-schema<br/>--output-format json"]

    CallClaude --> Parse[Parse JSON Response<br/>+ Validate Schema]
    Parse --> Store[Save to SQLite<br/>commit_analyses table]
    Store --> Delay["Rate limit delay<br/>(default 1s)"]
    Delay --> Next{More commits?}
    Next -->|Yes| Partition
    Next -->|No| Summary([Print Summary<br/>categories, cost, errors])
```

---

## Upstream Sync Pipeline

```mermaid
flowchart TD
    Start([commitflow upstream scan]) --> LoadCfg[Load upstream config<br/>from SQLite]
    LoadCfg --> ListUpstream["List upstream commits<br/>git rev-list from..to"]
    ListUpstream --> Filter{Already scanned?}

    Filter -->|Yes| SkipScanned[Skip]
    Filter -->|No| GetDiff[Get commit diff]

    GetDiff --> FileOverlap{Files overlap<br/>with fork?}
    FileOverlap -->|No overlap| NotApplicable["Mark: not_applicable<br/>(no AI call)"]
    FileOverlap -->|Has overlap| GatherCtx[Gather Fork Context<br/>overlapping files, fork-only dirs]

    GatherCtx --> BuildPrompt[Build Applicability Prompt]
    BuildPrompt --> CallAI["Claude CLI<br/>+ ApplicabilitySchema"]
    CallAI --> ParseResult[Parse & Validate Result]

    ParseResult --> RecordStatus{AI Status?}
    RecordStatus -->|applicable| Applicable[Save: applicable]
    RecordStatus -->|not_applicable| NA[Save: not_applicable]
    RecordStatus -->|needs_review| Review[Save: needs_review]
    RecordStatus -->|already_applied| Applied[Save: already_applied]

    Applicable --> UpdateHash[Update last_synced_hash]
    NA --> UpdateHash
    Review --> UpdateHash
    Applied --> UpdateHash
    NotApplicable --> UpdateHash

    UpdateHash --> Next{More commits?}
    Next -->|Yes| Filter
    Next -->|No| Summary([Print Scan Summary])
```

---

## Cherry-Pick Apply Flow

```mermaid
flowchart TD
    Start([commitflow upstream apply hash]) --> Resolve[Resolve sync record<br/>from SQLite]
    Resolve --> ValidateStatus{Status OK?}

    ValidateStatus -->|applied| ErrApplied[Error: already applied]
    ValidateStatus -->|not_applicable| ErrNA[Error: not applicable]
    ValidateStatus -->|applicable / needs_review| CreateBranch["git checkout -b<br/>upstream/hash[:8]"]

    CreateBranch --> CherryPick["git cherry-pick hash"]
    CherryPick --> Result{Success?}

    Result -->|Yes| GetHead["git rev-parse HEAD"]
    GetHead --> MarkApplied["Update DB:<br/>status=applied<br/>applied_commit=HEAD"]
    MarkApplied --> Done([Applied successfully])

    Result -->|Conflict| ListConflicts["git diff --name-only<br/>--diff-filter=U"]
    ListConflicts --> Abort["git cherry-pick --abort"]
    Abort --> MarkConflict["Update DB:<br/>status=conflict"]
    MarkConflict --> ReportConflict([Report conflicted files])
```

---

## Data Model

```mermaid
erDiagram
    commit_analyses {
        int id PK
        text repo_path
        text commit_hash UK
        text parent_hash
        text author
        text author_email
        text commit_date
        text message
        text files_changed "JSON array"
        text diff_stats "JSON object"
        text category "enum: bug_fix, feature, ..."
        text summary
        text detailed_analysis
        int impact_score "1-10"
        bool breaking_changes
        text packages_affected "JSON array"
        text created_at
    }

    upstream_configs {
        int id PK
        text name UK
        text upstream_url
        text fork_path
        text last_synced_hash
        text created_at
    }

    upstream_syncs {
        int id PK
        int config_id FK
        text upstream_commit UK
        text status "pending|applicable|applied|..."
        text applicability_reason
        int relevance_score "1-10"
        text applied_at
        text applied_commit
        text skip_reason
        text created_at
    }

    ai_call_log {
        int id PK
        text call_type "analysis|applicability"
        text commit_hash
        text model
        int input_tokens
        int output_tokens
        real cost_usd
        int duration_ms
        text created_at
    }

    upstream_configs ||--o{ upstream_syncs : "has many"
```

---

## Package Dependency Graph

```mermaid
graph LR
    main --> cmd
    cmd --> cmd_upstream["cmd/upstream"]
    cmd --> analysis
    cmd --> ai
    cmd --> git
    cmd --> storage
    cmd --> config

    cmd_upstream --> ai
    cmd_upstream --> git
    cmd_upstream --> storage
    cmd_upstream --> sync
    cmd_upstream --> config

    analysis --> ai
    analysis --> git
    analysis --> storage

    sync --> ai
    sync --> git
    sync --> storage

    git --> validate
    storage --> validate

    ai -.-> Claude["Claude CLI<br/>(external)"]
    git -.-> GitBin["git binary<br/>(external)"]
    storage -.-> SQLite["SQLite<br/>(external)"]
    config -.-> YAML["config.yaml<br/>(file)"]
```

---

## AI Client Retry & Budget Flow

```mermaid
flowchart TD
    Call([Client.Call]) --> BudgetCheck{Accumulated cost<br/>>= budget limit?}
    BudgetCheck -->|Yes| ErrBudget[Return ErrBudgetExhausted]
    BudgetCheck -->|No limit or OK| DryRun{Dry run?}
    DryRun -->|Yes| ReturnNil[Return nil]
    DryRun -->|No| Attempt["Execute claude CLI<br/>(attempt N)"]

    Attempt --> Success{Success?}
    Success -->|Yes| AccumCost[Accumulate cost<br/>thread-safe mutex]
    AccumCost --> RateLimit["Sleep delay<br/>(default 1s)"]
    RateLimit --> Return([Return Response])

    Success -->|Auth error| ErrAuth[Return ErrClaudeAuth<br/>no retry]
    Success -->|Other error| Retry{Retries left?}
    Retry -->|Yes| Backoff["Exponential backoff<br/>5s, 10s, 20s..."]
    Backoff --> Attempt
    Retry -->|No| ErrFinal[Return final error]
```

---

## Configuration Loading Priority

```mermaid
flowchart LR
    Defaults["DefaultConfig()"] --> YAML["YAML File<br/>~/.config/.../config.yaml"]
    YAML --> Env["Environment Variables<br/>GUT_MODEL, GUT_BUDGET, ..."]
    Env --> Flags["CLI Flags<br/>--model, --budget, ..."]
    Flags --> Final["Final Config"]

    style Defaults fill:#e8e8e8
    style YAML fill:#d4e6f1
    style Env fill:#d5f5e3
    style Flags fill:#fadbd8
    style Final fill:#f9e79f
```

*Rightmost source wins when values conflict.*
