# Ideas for Features

This document captures potential future features and enhancements for Choo.

## Docker Container Sandboxing for Unit Worktrees

**Idea:** Execute spec unit worktrees in Docker containers for enhanced sandboxing and isolation.

### Motivation
- Provide stronger isolation between parallel unit executions
- Prevent units from interfering with each other or the host system
- Enable reproducible execution environments
- Support different runtime requirements per unit (e.g., different Go versions, dependencies)

### Considerations
- Container lifecycle management (create, start, stop, cleanup)
- Volume mounting for worktree directories
- Network isolation vs. connectivity requirements (e.g., GitHub API access)
- Resource limits (CPU, memory) per container
- Integration with existing worktree management in `internal/git/`
- Performance overhead vs. benefits
- Configuration for container images (base images, custom Dockerfiles per unit)

### Potential Implementation
- Extend `internal/worker/` to support containerized execution
- Add Docker client integration
- Configurable per-unit or global container settings in `.choo.yaml`
- Fallback to non-containerized execution for compatibility

---

## User Interaction via Notifications (MCP Server / CLI Command)

**Idea:** Enable Claude code running in Choo to ask questions of the user via notifications through Slack, Telegram, or even phone calls.

### Motivation
- Allow autonomous agents to request human input when blocked or uncertain
- Enable real-time communication between running tasks and the user
- Support asynchronous workflows where immediate user response isn't required
- Provide multiple notification channels for different urgency levels

### Considerations
- **MCP Server Integration:** Expose as an MCP server so Claude Desktop and other MCP clients can use it
- **CLI Command:** Provide `choo ask` or similar command for programmatic access
- **Notification Channels:**
  - Slack (webhooks, bot API)
  - Telegram (bot API)
  - Email
  - SMS (via Twilio or similar)
  - Phone calls (via Twilio Voice API)
  - Push notifications (if mobile app exists)
- **Question Format:** Structured questions with:
  - Question text
  - Options/choices (if applicable)
  - Urgency level
  - Timeout/expiration
  - Context about what task is asking
- **Response Handling:** 
  - How to receive and route responses back to the asking task
  - Response storage/persistence
  - Timeout handling
- **Configuration:** Per-channel credentials and settings in `.choo.yaml` or environment variables
- **Security:** Authentication/authorization for responses
- **Integration Points:**
  - Worker execution context
  - Ralph loop integration
  - Event bus for question/response events

### Potential Implementation
- New package `internal/notifications/` for channel abstractions
- MCP server implementation in `internal/mcp/` or separate service
- CLI command in `internal/cli/`
- Event types in `internal/events/` for question/response lifecycle
- Configuration schema in `specs/CONFIG.md`

### Example Use Cases
- Task needs approval before making a breaking change
- Task is blocked and needs user input on approach
- Task needs credentials or API keys
- Task wants to confirm before proceeding with expensive operation
- Task needs clarification on ambiguous requirements

---

## Non-Software Development Workflows (Skill/Procedure Heavy)

**Idea:** Extend Choo to support non-software development tasks that follow runbooks and procedures, such as financial reconciliation, transaction categorization for tax purposes, and other knowledge-work automation.

### Motivation
- Apply Choo's orchestration and parallelization capabilities to business process automation
- Enable autonomous execution of procedural tasks that require decision-making
- Support workflows that combine code (MCP tools, integrations) with human knowledge (runbooks, procedures)
- Create reusable skill/procedure libraries for common business tasks

### Use Cases
- **Financial Reconciliation:** Reconcile bank accounts, credit cards, investment accounts
- **Transaction Categorization:** Categorize transactions for tax purposes following IRS rules and personal preferences
- **Document Processing:** Process invoices, receipts, statements following organizational procedures
- **Compliance Tasks:** Execute compliance checklists, audit procedures, regulatory reporting
- **Data Entry & Validation:** Automated data entry with validation rules and exception handling

### Key Characteristics
- **Runbook-Driven:** Tasks follow documented procedures/runbooks rather than code specifications
- **Skill/Procedure Heavy:** Repository contains more procedural knowledge (markdown runbooks, decision trees) than code
- **Hybrid Code + Knowledge:** Still includes code for:
  - MCP tools (e.g., bank API integrations, accounting software APIs)
  - Data processing utilities
  - Validation logic
  - Integration with external systems
- **Interactive Decision Points:** Agent can ask user questions when:
  - Runbook is ambiguous or incomplete
  - Multiple valid categorization options exist
  - Exception cases are encountered
  - User judgment is required

### Considerations

#### Repository Structure
- How to organize skill/procedure specs vs. code?
- Should runbooks live in `specs/` alongside task specs?
- Directory structure for procedure libraries (e.g., `procedures/finance/`, `procedures/tax/`)?
- Relationship between procedural knowledge and executable code

#### Workflow Design
- **Runbook Format:** What format for runbooks? (Markdown with structured sections? YAML? Custom DSL?)
- **Decision Points:** How to model conditional logic and decision trees in runbooks?
- **State Management:** How to track progress through multi-step procedures?
- **Data Flow:** How do procedures reference and transform data (CSV files, API responses, etc.)?
- **Validation Rules:** Where do validation rules live? (In runbooks? Separate config? Code?)

#### Integration with Existing Choo Concepts
- **Units:** What constitutes a "unit" in a procedural context? (A reconciliation task? A categorization batch?)
- **Tasks:** How do procedural steps map to Choo tasks?
- **Dependencies:** How to model dependencies between procedural steps?
- **Backpressure:** What constitutes "done" for a procedural task? (All items processed? User approval?)
- **Worktrees:** Do procedural tasks need worktrees? (Maybe for data isolation, audit trails?)

#### User Interaction
- **Question Types:** 
  - Binary choices (yes/no, approve/reject)
  - Multiple choice (select category, select rule to apply)
  - Free-form input (notes, explanations)
  - File uploads (receipts, supporting documents)
- **Context Presentation:** How to show relevant context when asking questions? (Transaction details, previous similar cases, runbook section)
- **Batch Questions:** Can agent batch similar questions for efficiency?
- **Learning:** Can agent learn from user responses to reduce future questions?

#### Data Handling
- **Input Sources:** CSV files, API integrations, database queries, manual uploads
- **Data Privacy:** Sensitive financial data requires careful handling
- **Audit Trails:** Full logging of decisions, changes, user interactions
- **Versioning:** Track changes to categorized data, reconciliation results

### Potential Implementation

#### Repository Structure
```
choo-repo/
├── procedures/           # Procedural knowledge
│   ├── finance/
│   │   ├── reconciliation.md
│   │   └── categorization-rules.md
│   └── tax/
│       └── transaction-categories.md
├── tools/                # MCP tools and integrations
│   ├── bank-api/
│   └── accounting-software/
├── specs/                # Task specs (as today)
│   └── tasks/
│       └── monthly-reconciliation/
└── data/                 # Input/output data (gitignored?)
    ├── transactions/
    └── results/
```

#### Runbook Format (Example)
```markdown
# Transaction Categorization Runbook

## Rules
1. If merchant matches pattern "STARBUCKS", categorize as "Food & Dining > Coffee"
2. If amount > $500 and no receipt, ask user for category
3. If transaction date is in tax year, apply tax categorization rules

## Decision Tree
- Is transaction a transfer? → Skip categorization
- Is merchant in known list? → Use predefined category
- Does amount match pattern? → Apply rule
- Otherwise → Ask user

## Exception Handling
- If uncertain, present top 3 likely categories to user
- Include context: similar past transactions, merchant info, amount
```

#### Extensions to Choo
- **Procedure Discovery:** Extend `internal/discovery/` to parse runbooks/procedures
- **Interactive Worker:** Extend `internal/worker/` to handle question/answer loops
- **Data Tools:** MCP tools for reading/writing structured data (CSV, JSON, etc.)
- **Procedure Engine:** New package `internal/procedures/` for executing runbooks
- **Question Queue:** Integration with notification system for user questions

### Open Questions
- Should procedural tasks use the same Unit/Task model as code tasks?
- How to version and evolve runbooks over time?
- Can procedures call other procedures (composition)?
- How to test/validate procedural workflows?
- Should there be a visual workflow builder for complex procedures?
- How to handle partial completion and resumption of procedural tasks?
