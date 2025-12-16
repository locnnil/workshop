```{eval-rst}
:orphan:

.. meta::
   :description: Workshop Go coding style guide covering error handling, naming
                 conventions, code structure, testing patterns, and architecture
                 principles derived from PR discussions and contribution standards.
```

# Workshop Go coding style guide

This style guide documents Go-specific coding conventions used in the Workshop project. It captures patterns from code review discussions in merged PRs and established project standards. These guidelines complement Canonical's general coding standards with Workshop-specific decisions.

The guide is evidence-based, derived from actual PR discussions between maintainers (primarily @dmitry-lyfar and @jonathan-conder) during code reviews.

---

## Error handling

### Error message format

**Pattern**: Error messages start lowercase, contain no trailing punctuation, and follow the template: "what was attempted: why it went wrong".

**Rationale**: Maintains consistency with existing error handling patterns and provides clear, actionable user guidance.

**Good**:

```go
// From cmd/workshop/connect.go
return fmt.Errorf("cannot connect plugs and slots across different workshops")

// From cmd/workshop/list.go
return fmt.Errorf("cannot list: \"--project\" incompatible with \"--global\"")

// From internal/daemon/api_workshops.go
return statusBadRequest("project-id required")
```

**Avoid**:

```go
return fmt.Errorf("Cannot connect plugs.") // Starts with capital, has punctuation
return fmt.Errorf("Error") // Not descriptive enough
```

**Reference**: PR #257, `docs/contributing.rst` Error Messages section

---

### Consistent error handling pattern

**Pattern**: Use one of two standard patterns consistently throughout the codebase.

**For simple function calls**:

```go
if err := f(); err != nil {
    return err
}
```

**For functions with multiple returns**:

```go
val, err := f()
if err != nil {
    return err
}
```

**Examples from codebase**:

```go
// From cmd/workshopd/run.go
if err := dirs.CreateDirs(); err != nil {
    return err
}

// From cmd/sdk/main.go
if err := cmd.Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

**Rationale**: Consistent error handling improves code readability and maintainability.

**Reference**: `docs/contributing.rst` Coding Standards, PR #262 (errcheck linter)

---

### Explicit error checking

**Pattern**: Always check and handle errors. Use `_ = someFunc()` to explicitly ignore intentionally.

**Good**:

```go
// Explicitly discarding error
_ = file.Close()

// Handling error
if err := file.Close(); err != nil {
    log.Printf("failed to close file: %v", err)
}
```

**Avoid**:

```go
file.Close() // Unchecked error
```

**Rationale**: The errcheck linter is enabled for new code to catch unhandled errors. Explicit `_` assignment shows intentional discard.

**Reference**: PR #262

---

## Naming conventions

### Function names reflect behavior accurately

**Pattern**: Function names must accurately describe what the function does. Avoid ambiguous prefixes like "maybe" when the action is mandatory.

**Good**:

```go
func reloadSystemd() error {
    // Reload is mandatory
}

func connectIfAvailable() error {
    // "IfAvailable" indicates conditional behavior
}
```

**Avoid**:

```go
func maybeReloadSystemd() error {
    // Confusing - reload is actually mandatory
}
```

**Rationale**: Prevents confusion about function behavior and side effects. The "maybe" prefix should only be used for truly optional operations.

**Reference**: PR #257 review comment on `maybeReloadSystemd`

---

### Descriptive variable names

**Pattern**: Use names that reflect purpose or filtering intent, not generic permission terms.

**Good**:

```go
statusFilter := []string{"Ready", "Error"}
matchesStatus := func(s string) bool {
    if statusFilter == nil {
        return true
    }
    return slices.Contains(statusFilter, s)
}
```

**Avoid**:

```go
allowed := []string{"Ready", "Error"} // Too generic
```

**Rationale**: Improves code clarity and communicates intent.

**Reference**: PR #275 review discussion

---

### Test constant naming

**Pattern**: Test constants should have sensible, descriptive names that reflect their purpose.

**Good**:

```go
const (
    testProjectID   = "test-project-123"
    testWorkshopName = "dev-workshop"
    fakeAPIResponse = `{"status": "ready"}`
)
```

**Avoid**:

```go
const (
    s1 = "test-project-123"
    ws = "dev-workshop"
)
```

**Rationale**: Improves test readability and maintainability.

**Reference**: PR #273

---

## Code structure and organization

### Complete related operations before moving to next attribute

**Pattern**: When processing multiple attributes, finish all operations related to one attribute before moving to the next.

**Good**:

```go
// Process target attribute completely
target := plugAttrs["target"]
if err := validatePath(target); err != nil {
    return err
}
parsedTarget := parsePath(target)

// Now move to next attribute
source := plugAttrs["source"]
// ... process source
```

**Avoid**:

```go
// Getting target
target := plugAttrs["target"]

// Getting source
source := plugAttrs["source"]

// Validating target (separated from getting it)
if err := validatePath(target); err != nil {
    return err
}
```

**Rationale**: Improves code clarity by maintaining logical grouping of operations. Related code stays together.

**Reference**: PR #257 review on `mount.go`

---

### Extract common logic into reusable functions

**Pattern**: Extract duplicated completion logic into reusable functions rather than duplicating inline.

**Good**:

```go
func completeWorkshopName(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    // Common workshop name completion logic
    return workshops, cobra.ShellCompDirectiveNoFileComp
}

cmd := &cobra.Command{
    ValidArgsFunction: completeWorkshopName,
}
```

**Avoid**:

```go
cmd := &cobra.Command{
    ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
        // Inline completion logic duplicated across commands
        cli, err := root.client()
        // ... 30 lines of logic ...
        return workshops, cobra.ShellCompDirectiveNoFileComp
    },
}
```

**Rationale**: Reduces duplication, avoids adding long inline functions into Cobra Command structure initialization, improves maintainability.

**Reference**: PR #275, PR #302

---

### Prefer existing attributes over re-iterating

**Pattern**: Use existing object attributes instead of re-iterating collections when the information is already available.

**Good**:

```go
for _, plug := range plugs {
    if len(plug.Connections) > 0 {
        // Plug is connected
    }
}
```

**Avoid**:

```go
for _, plug := range plugs {
    isConnected := false
    for _, conn := range allConnections {
        if conn.Plug.Name == plug.Name {
            isConnected = true
            break
        }
    }
}
```

**Rationale**: Improves readability and reduces unnecessary iterations when data is already present in objects.

**Reference**: PR #275

---

### Code should reflect logical flow clearly

**Pattern**: Structure code to match its logical intent — filter first, then transform.

**Good**:

```go
// Filter connected plugs
var connectedPlugs []Plug
for _, plug := range allPlugs {
    if len(plug.Connections) > 0 {
        connectedPlugs = append(connectedPlugs, plug)
    }
}

// Transform to suggestions
for _, plug := range connectedPlugs {
    suggestions = append(suggestions, plug.ToCompletion())
}
```

**Avoid**:

```go
// Mixed filtering and transformation
for _, plug := range allPlugs {
    if len(plug.Connections) > 0 {
        suggestions = append(suggestions, plug.ToCompletion())
    }
}
```

**Rationale**: Makes intention explicit and improves readability for simple logic.

**Reference**: PR #275 discussion on filtering connected plugs

---

### Use reference comparison over endpoint comparison

**Pattern**: Use `Ref()` comparison when project IDs matter, not endpoint strings.

**Good**:

```go
if plug.Ref() == conn.Plug.Ref() {
    // Correct comparison including project ID
}
```

**Avoid**:

```go
plugEndpoint := endpoint(plug.Workshop, plug.SDK, plug.Name)
connEndpoint := endpoint(conn.Plug.Workshop, conn.Plug.SDK, conn.Plug.Name)
if plugEndpoint == connEndpoint {
    // Misses project ID differences
}
```

**Rationale**: Endpoints don't include project IDs, which can cause incorrect matching when the same endpoint exists in different projects.

**Reference**: PR #275

---

## Type handling

### Use type switches for multiple possible types

**Pattern**: When handling multiple possible input types, use type switches with explicit error messages for each case.

**Good**:

```go
switch ro := readOnly.(type) {
case bool:
    return ro, nil
case string:
    parsed, err := strconv.ParseBool(ro)
    if err != nil {
        return false, fmt.Errorf("invalid boolean string %q", ro)
    }
    return parsed, nil
default:
    return false, fmt.Errorf("read-only must be bool or string, got %T", ro)
}
```

**Avoid**:

```go
if b, ok := readOnly.(bool); ok {
    return b, nil
}
if s, ok := readOnly.(string); ok {
    // ... parse string
}
// No clear error for other types
```

**Rationale**: Provides better error reporting and makes code more maintainable. This is the established pattern used throughout the codebase for handling multiple types.

**Examples from codebase**:

```go
// From internal/asserts/constraint.go
switch x := v.(type) {
case string:
    return x, nil
case int:
    return strconv.Itoa(x), nil
default:
    return "", fmt.Errorf("invalid type %T", v)
}
```

**Reference**: PR #257, internal/asserts patterns

---

### Avoid generics when concrete types are consistent

**Pattern**: Don't use generics when type variation doesn't actually exist.

**Good**:

```go
func filterByStatus(items []Workshop, status string) []Workshop {
    // Concrete types used consistently
}
```

**Avoid**:

```go
func filterByStatus[T any](items []T, status string) []T {
    // Unnecessary generics when T is always Workshop
}
```

**Rationale**: Simplifies code when type variation doesn't exist in practice.

**Reference**: PR #275

---

## Testing patterns

### Test with JSON response mocking, not ad-hoc interfaces

**Pattern**: In command packages, test API interactions by mocking JSON HTTP responses, not by creating ad-hoc client interfaces.

**Good**:

```go
// In cmd/workshop/connect_test.go
func (s *connectSuite) TestConnect(c *check.C) {
    s.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(ConnectionsResponse{
            Plugs: []Plug{{Name: "test"}},
        })
    })
    
    err := cmdConnect.Run(cmdConnect.Command(), []string{"test"})
    c.Assert(err, check.IsNil)
}
```

**Avoid**:

```go
// Creating ad-hoc interface in cmd package
type Client interface {
    Connections() ([]Connection, error)
}

func (c *CmdConnect) SetClient(cli Client) {
    c.client = cli
}
```

**Rationale**: Keeps interface definitions in appropriate packages (client library) and maintains architectural boundaries. Command packages should focus on CLI logic, not define their own client interfaces.

**Reference**: PR #275 review comments

---

### Use real test data, not faked data

**Pattern**: Tests should use realistic data structures that match actual API responses.

**Good**:

```go
testWorkshop := Workshop{
    Name:   "dev",
    Base:   "ubuntu@22.04",
    Status: "Ready",
    SDKs: []SDK{
        {Name: "go", Channel: "22.04/stable"},
    },
}
```

**Avoid**:

```go
// Overly simplified fake data
testWorkshop := Workshop{
    Name: "test",
}
```

**Rationale**: Real data catches edge cases and integration issues that simplified fakes miss.

**Reference**: PR #254

---

### Minimize duplication in test setup

**Pattern**: Extract common test setup into helper functions or shared constants, but allow some duplication for clarity when needed.

**Good**:

```go
const readyWorkshopJSON = `{
    "name": "dev",
    "status": "Ready"
}`

func (s *testSuite) setupReadyWorkshop(c *check.C) Workshop {
    return Workshop{Name: "dev", Status: "Ready"}
}
```

**Avoid excessive coupling**:

```go
// Reusing status across unrelated tests
const sharedStatus = "Ready" // Used for both success and error cases
```

**Rationale**: Balance between DRY and test clarity. Some duplication is acceptable in tests to keep them self-contained and understandable.

**Reference**: PR #289 review on test constants

---

## Architecture and separation of concerns

### Sorting belongs in representation layer, not client library

**Pattern**: Client libraries should focus on data retrieval. Sorting and presentation logic belongs in the command/UI layer.

**Good**:

```go
// In client library
func (c *Client) Changes() ([]Change, error) {
    // Just retrieve and return data
}

// In cmd package
func (c *CmdChanges) Run(cmd *cobra.Command, args []string) error {
    changes, err := c.client.Changes()
    if err != nil {
        return err
    }
    
    // Sort for presentation
    sort.Slice(changes, func(i, j int) bool {
        return changes[i].ID > changes[j].ID
    })
}
```

**Avoid**:

```go
// In client library
func (c *Client) Changes() ([]Change, error) {
    changes, err := c.fetch()
    sort.Slice(changes, func(i, j int) bool {
        return changes[i].ID > changes[j].ID
    })
    return changes, err
}
```

**Rationale**: Separates data access from presentation concerns, making the client library reusable for different presentation needs.

**Reference**: PR #275 discussion on sorting in client vs command layer

---

## Nil handling patterns

### Use nil checks for "accept all" semantics

**Pattern**: Use nil to represent "accept all" filtering, and extract into named functions for clarity.

**Good**:

```go
matchesStatus := func(s string) bool {
    if status == nil {
        return true // nil means accept all
    }
    return slices.Contains(status, s)
}

for _, workshop := range workshops {
    if matchesStatus(workshop.Status) {
        results = append(results, workshop)
    }
}
```

**Avoid**:

```go
for _, workshop := range workshops {
    if status == nil || slices.Contains(status, workshop.Status) {
        // Inline logic less clear
        results = append(results, workshop)
    }
}
```

**Rationale**: Makes the "accept all" intent explicit and improves readability.

**Reference**: PR #275

---

## Code quality principles

### Blank lines for logical separation

**Pattern**: Insert blank lines between logically different sections of code.

**Good**:

```go
func process() error {
    // Validation section
    if name == "" {
        return fmt.Errorf("name required")
    }
    if id == "" {
        return fmt.Errorf("id required")
    }

    // Data transformation section
    normalized := strings.ToLower(name)
    formatted := fmt.Sprintf("%s-%s", normalized, id)

    // Persistence section
    if err := save(formatted); err != nil {
        return err
    }

    return nil
}
```

**Rationale**: Improves code structure and makes it easier to understand different logical sections.

**Reference**: `docs/contributing.rst` Coding Standards

---

### Avoid nested conditions

**Pattern**: Use early returns to reduce nesting levels.

**Good**:

```go
func validate(workshop *Workshop) error {
    if workshop == nil {
        return fmt.Errorf("workshop is nil")
    }
    
    if workshop.Name == "" {
        return fmt.Errorf("name required")
    }
    
    if !isValidBase(workshop.Base) {
        return fmt.Errorf("invalid base")
    }
    
    return nil
}
```

**Avoid**:

```go
func validate(workshop *Workshop) error {
    if workshop != nil {
        if workshop.Name != "" {
            if isValidBase(workshop.Base) {
                return nil
            } else {
                return fmt.Errorf("invalid base")
            }
        } else {
            return fmt.Errorf("name required")
        }
    } else {
        return fmt.Errorf("workshop is nil")
    }
}
```

**Rationale**: Reduces cognitive load, improves readability, and makes the happy path clearer.

**Reference**: `docs/contributing.rst` Coding Standards

---

### Delete dead code and redundant comments

**Pattern**: Remove unused code and comments that don't add value.

**Good**:

```go
func process() error {
    // Handle special case for empty input
    if input == "" {
        return nil
    }
    
    return transform(input)
}
```

**Avoid**:

```go
func process() error {
    // TODO: implement this later
    // Legacy code from old implementation
    // input := getOldInput()
    
    // Get input
    input := getInput()
    // Check if empty
    if input == "" {
        // Return nil
        return nil
    }
    
    // Transform the input
    return transform(input)
}
```

**Rationale**: Keeps codebase clean and maintainable. Redundant comments add noise without value.

**Reference**: `docs/contributing.rst` Self-Review Quick Check

---

### Normalize symmetries

**Pattern**: Handle identical operations identically throughout the codebase.

**Good**:

```go
// Consistent error handling pattern everywhere
if err := validateName(name); err != nil {
    return err
}

if err := validateBase(base); err != nil {
    return err
}

if err := validateSDKs(sdks); err != nil {
    return err
}
```

**Avoid**:

```go
// Inconsistent handling
if err := validateName(name); err != nil {
    return err
}

err := validateBase(base)
if err != nil {
    return err
}

if validateSDKs(sdks) != nil {
    return validateSDKs(sdks) // Called twice!
}
```

**Rationale**: Consistency improves maintainability and reduces cognitive load when reading code.

**Reference**: `docs/contributing.rst` Coding Standards

---

## Project-specific patterns

### Cobra command structure

**Pattern**: Don't inline long functions in cobra.Command initialization. Extract ValidArgsFunction and RunE implementations.

**Good**:

```go
func newCmdConnect() *cobra.Command {
    c := &CmdConnect{}
    
    cmd := &cobra.Command{
        Use:               "connect",
        RunE:              c.Run,
        ValidArgsFunction: c.complete,
    }
    
    return cmd
}

func (c *CmdConnect) Run(cmd *cobra.Command, args []string) error {
    // Implementation
}

func (c *CmdConnect) complete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    // Completion implementation
}
```

**Avoid**:

```go
func newCmdConnect() *cobra.Command {
    cmd := &cobra.Command{
        Use: "connect",
        RunE: func(cmd *cobra.Command, args []string) error {
            // 50 lines of inline implementation
        },
        ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
            // 30 lines of inline completion logic
        },
    }
    
    return cmd
}
```

**Rationale**: Keeps command initialization clean and functions testable. Matches the pattern established in PR #275 and PR #302.

**Reference**: PR #275, PR #302, PR #261, PR #263

---

## References

This guide was derived from:

- **PR discussions**: #257, #262, #269, #275, #289, #302 (primary sources)
- **Contributing guide**: `docs/contributing.rst`
- **Codebase patterns**: Observed in `cmd/`, `internal/`, `client/` packages

### Key contributors to these patterns

- @dmitry-lyfar (primary reviewer)
- @jonathan-conder (contributor and reviewer)
- @akcano (documentation and style refinement)
- @lachypjones (contributor)

### Related documentation

- [Contributing Guide](contributing.rst) — Setup, workflow, and general standards
- [Documentation Style Guide](doc-style-guide.md) — Documentation-specific conventions

---

## Evolution note

These guidelines evolve with the project. When reviewing code:

1. Reference this guide in code reviews
2. Propose updates when new patterns emerge
3. Keep guidelines evidence-based from actual PR discussions
4. Update with links to specific PRs that establish new patterns
