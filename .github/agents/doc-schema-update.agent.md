Title: Reconcile workshop_file.go Validation Logic with schema.json

Mission:
Ensure that the JSON schema at docs/reference/definition-files/schema.json precisely reflects the validation and parsing rules enforced in internal/workshop/workshop_file.go, correcting only those discrepancies where the Go verification logic clearly requires a different mandatory structure or constraint than what the schema currently defines.

Context:
- You are working inside a repository that contains:
  - JSON schema: docs/reference/definition-files/schema.json
  - Go validation/verification logic: internal/workshop/workshop_file.go
- The Go code in workshop_file.go is treated as the single source of truth for what is currently enforced at runtime.
- The schema.json file is used to describe and validate the same document format (definition files) that workshop_file.go parses and verifies.

Task Requirements:
1. Use internal/workshop/workshop_file.go as the authoritative specification of required fields, types, and constraints.
2. Compare the structure, required/optional fields, allowed values, and type constraints in workshop_file.go against those defined in docs/reference/definition-files/schema.json.
3. Identify all cases where:
   - The Go code requires a field or constraint that the schema does not mark as required or does not constrain correctly.
   - The Go code enforces stricter or different structural rules than the schema indicates (e.g., enums, nested structures, array element shapes, disallowed values, or mutually exclusive fields).
4. Update docs/reference/definition-files/schema.json ONLY when:
   - There is a direct, verifiable mismatch where workshop_file.go makes something mandatory or constrained, and the schema fails to express that requirement.
   - The change is necessary so that a document valid per schema.json cannot contradict the rules enforced by workshop_file.go.
5. Do NOT modify any other files under any circumstances:
   - Do not change internal/workshop/workshop_file.go.
   - Do not change other schemas, documentation, or configuration files.
   - Do not introduce new helper files or scripts.

Process / Steps:
1. Code Analysis in workshop_file.go:
   - Locate all parsing and validation entry points and data structures related to the documents that schema.json describes.
   - Identify the Go struct(s) and types used to represent the schema’s data model (e.g., structs, embedded structs, maps, slices).
   - Enumerate:
     - All fields, their Go types, and JSON/YAML tags.
     - Whether each field is required, optional, or has default behaviors.
     - Any validation logic (e.g., length checks, allowed values, non-empty constraints, cross-field dependencies).
   - Pay special attention to:
     - Fields that must be present (no nil / zero-value allowed).
     - Fields that are conditionally required based on other fields.
     - Expected enum-like fields and constant sets.
     - Structural invariants: required nested objects, non-empty arrays, unique keys.

2. Schema Analysis in schema.json:
   - Open docs/reference/definition-files/schema.json and identify:
     - The top-level type and its properties.
     - The "required" lists at each object level.
     - Type definitions: type, format, enum, minItems, maxItems, pattern, additionalProperties, etc.
     - Any $ref, allOf/anyOf/oneOf, and nested object definitions.
   - Map each schema property (and nested property) to its corresponding field/validation rule in workshop_file.go.

3. Cross-Reference & Discrepancy Detection:
   - For each field or structure defined in workshop_file.go:
     - Confirm it exists in schema.json with consistent:
       - Name (as per JSON/YAML tag).
       - Type (string vs number vs boolean vs object vs array).
       - Required/optional status.
       - Constraints (e.g., enums, non-empty arrays, uniqueness, scalar ranges).
   - Record discrepancies, including:
     - Fields that are required in Go but not listed as required in schema.json.
     - Fields whose type in schema.json does not match the Go type (or effective type) used in workshop_file.go.
     - Missing constraints in schema.json where workshop_file.go performs explicit checks (e.g., non-empty, limited set of values).
     - Structural rules enforced in Go (e.g., disallowing empty lists, requiring certain subfields if a parent field is present) that the schema omits.

4. Decide Which Discrepancies Warrant Schema Updates:
   - Apply the conservative principle:
     - If workshop_file.go requires something and schema.json is looser → you MUST tighten the schema to match the enforced requirement.
     - If workshop_file.go allows something but schema.json is more restrictive → LEAVE the schema as-is unless the mismatch would clearly break valid, real-world documents or contradicts the documented, intended behavior.
   - Do NOT introduce new optional fields or relax existing constraints just to make the schema “more permissive.”
   - Do NOT add new behaviors that are not explicitly enforced by workshop_file.go.

5. Implement Changes in schema.json:
   - Modify ONLY docs/reference/definition-files/schema.json.
   - For each discrepancy that must be fixed:
     - Adjust the "required" arrays to include fields that the Go code mandates.
     - Update property definitions to match the true type (e.g., array vs object vs scalar, enum vs free-form).
     - Add or update constraints (e.g., enum lists, minItems, pattern) when workshop_file.go enforces them.
     - Preserve existing structure and naming; do not perform cosmetic refactors or reorganizations unless strictly necessary to express the enforced rule.
   - Avoid any broad refactoring of the schema structure; keep changes localized and minimal.

6. Local Verification (if tools are available in the environment):
   - If applicable, run:
     - go test, or the repository-specific test command, to validate that the changes do not introduce regressions.
     - Any existing schema validation or CI helper tools that may be available.
   - Validate at least a small set of sample documents (if present in the repo) against the updated schema to ensure that documents already accepted by workshop_file.go remain valid.

Constraints & Guardrails:
1. Source of Truth:
   - Treat internal/workshop/workshop_file.go as the authoritative definition of enforced behavior.
   - Do not “correct” the Go code via assumptions in the schema; if a behavior looks odd but is clearly enforced, reflect it in the schema rather than changing semantics.
2. Conservative Changes:
   - Make the smallest possible set of edits to schema.json necessary to reconcile it with workshop_file.go.
   - Do not:
     - Introduce new informational or optional fields.
     - Simplify, generalize, or redesign the schema.
     - Add documentation, comments, or examples in other files.
3. File Scope:
   - You may edit ONLY docs/reference/definition-files/schema.json.
   - No new files, no changes to Go source files, no modifications to other schemas or docs.
4. No Behavioral Expansion:
   - Do not use this task to add new features, new validation rules, or future-oriented constraints not currently enforced by workshop_file.go.

Output Specification:
Your final response must include:

1. Summary of Findings:
   - A concise list of all discrepancies you found between workshop_file.go and schema.json.
   - For each discrepancy, briefly describe:
     - What workshop_file.go enforces.
     - What schema.json allowed or required before your changes.

2. Explanation of Changes:
   - For every change you made to docs/reference/definition-files/schema.json:
     - Describe the change in natural language.
     - Reference the specific parts of workshop_file.go that justify this change (e.g., struct name and field, validation function, conditional checks).
     - Mention the relevant section(s) of schema.json (e.g., property path or section name).

3. Updated schema.json:
   - Provide the final, updated contents of docs/reference/definition-files/schema.json in full, or a precise unified diff, suitable for direct application (e.g., a git-style patch).
   - Ensure the JSON is syntactically valid, properly formatted, and preserves the existing formatting style as much as possible.

Quality Checks:
Before finalizing, explicitly verify that:
1. Every field that workshop_file.go treats as mandatory is now effectively required by schema.json at the appropriate nesting level.
2. All type and structural definitions in schema.json are consistent with how workshop_file.go parses and stores data.
3. Constraints enforced programmatically in workshop_file.go (such as allowed value sets, non-empty constraints, or dependent fields) are represented in schema.json wherever doing so:
   - Does not relax existing constraints.
   - Does not introduce new, non-existent behavior.
4. You have not:
   - Modified or referenced any files other than internal/workshop/workshop_file.go and docs/reference/definition-files/schema.json.
   - Introduced any new optional fields or loosened any existing constraints.
5. The updated schema.json is valid JSON and can be parsed without errors.
