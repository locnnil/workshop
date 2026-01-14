.. _how_agentic_coding:

.. meta::
   :description: How-to guide on using Workshop for parallel agentic coding,
                 demonstrating multi-agent workflows with 'claude-code' and
                 'copilot-cli' SDKs in a shared sandbox environment.

How to use workshops for agentic coding
=======================================

.. @artefact workshop (container)
.. @artefact workshop definition

|ws_markup| enables multiple tools and utilities,
brought together as SDKs,
to work in the same sandbox environment over a shared project repository,
thus addressing security and privacy concerns
while enabling a degree of creativity in your workflows.

This guide demonstrates how to run heterogeneous AI coding SDKs
in separate Git worktrees,
comparing their outputs
and synthesizing the best approach in a few different ways.
It walks through two scenarios
that build a Rubik's cube solver with 3D visualization:

- Scenario 1: Parallel exploration.
  Run :program:`claude-code` and :program:`copilot-cli` on the same task,
  then compare implementations and merge the best elements from both.

- Scenario 2: Role-based coding.
  Assign distinct roles to different agents:
  :program:`claude-code` as the architect, creating planning documents,
  :program:`copilot-cli` as the coder, implementing the design.


Prerequisites
-------------

This guide builds on :ref:`how_git_workshops` and :ref:`how_add_actions`,
particularly the use of Git worktrees with workshops.
Worktrees help isolate and track different agents' runs and outcomes
while sharing the same project repository.

Start with a fresh directory
and initialize a Git repository:

.. code-block:: console

   $ mkdir rubik-project && cd rubik-project
   $ git init


Note that :program:`claude-code` and :program:`copilot-cli` SDKs
may prompt for login credentials on first run.
Both support token-based API authentication via environment variables as well;
refer to the agents' respective documentation for setup details.


Agent prompts
-------------

Create a :file:`prompts/` directory at the repository root
to store all agent prompts:

.. code-block:: console

   $ mkdir prompts


Create the prompt for parallel exploration:

.. code-block:: markdown
   :caption: prompts/rubik-shared.md

   # Task: Build a Rubik's Cube Solver with 3D Visualization

   Create a complete Rubik's cube solver with 3D visualization in Python.

   Requirements:
   - Use a bulletproof 3D visualization library (matplotlib or pygame)
   - Implement a working solving algorithm
   - Code should run on Ubuntu Linux 24.04
   - Include testing to verify the solver works correctly

   The implementation should be complete and working.


Create the synthesis prompt:

.. code-block:: markdown
   :caption: prompts/rubik-synthesis.md

   # Task: Compare and Synthesize Rubik's Cube Implementations

   Two implementations of a Rubik's cube solver exist:
   - rubik-claude/ (claude-code implementation)
   - rubik-copilot/ (copilot-cli implementation)

   Compare both implementations across:
   - Code simplicity and readability
   - Architecture maturity
   - Overall implementation quality

   For each major design decision,
   ask the user which approach they prefer.
   Then synthesize the best elements from both
   into a final implementation in this directory.


Create the architect prompt for Scenario 2:

.. code-block:: markdown
   :caption: prompts/rubik-architect.md

   # Role: Software Architect

   Create planning documentation for a Rubik's cube solver project:

   1. tech-stack.md: Recommended Python libraries and justification
   2. design.md: Architecture, module structure, solving algorithm choice
   3. requirements.md: Functional requirements and constraints

   Focus on pragmatic choices that work reliably on Ubuntu Linux 24.04.


Create the coder prompt that references the planning documents:

.. code-block:: markdown
   :caption: prompts/rubik-coder.md

   # Role: Implementation Engineer

   Read the planning documents from ../design/ and implement a complete
   Rubik's cube solver with 3D visualization.

   Requirements:
   - Follow the architecture from ../design/design.md
   - Use tech stack from ../design/tech-stack.md
   - Meet requirements from ../design/requirements.md
   - Include tests to verify correctness
   - Ensure code runs on Ubuntu Linux 24.04

   The implementation should be complete and working.


.. note::

   These prompts are simplified examples.
   In practice, you may need more elaborate prompts
   or use metaprompting techniques to generate effective instructions
   for your specific situation.


Workshop definitions
--------------------

This guide uses two separate workshop definitions,
one for each AI coding SDK.
This approach isolates each agent's dependencies
and allows for SDK-specific configurations.

Create a workshop definition for :program:`claude-code`:

.. code-block:: yaml
   :caption: .workshop/claude-dev.yaml

   name: claude-dev
   base: ubuntu@24.04
   sdks:
     - name: claude-code
       channel: all/edge

   actions:
     run: |
       claude --dangerously-skip-permissions --print


Create a second workshop definition for :program:`copilot-cli`:

.. code-block:: yaml
   :caption: .workshop/copilot-dev.yaml

   name: copilot-dev
   base: ubuntu@24.04
   sdks:
     - name: copilot-cli
       channel: all/edge

   actions:
     run: |
       copilot --allow-all-tools  --allow-all-paths --silent --prompt


.. @artefact workshop base image
.. @artefact SDK

The :samp:`actions` section defines shell commands
that encapsulate the complexity of running different agents
with their specific options and idioms.
Each workshop exposes a :samp:`run` action
that invokes its respective SDK with appropriate flags.


Scenario 1: Parallel runs
-------------------------

This scenario runs two different AI agents
on the same task,
then uses a third agent
to compare implementations and synthesize the best approach.


First worktree
~~~~~~~~~~~~~~

Create a worktree for the first agent:

.. code-block:: console

   $ git worktree add rubik-claude
   $ cd rubik-claude/


.. @artefact workshop launch

Launch the :program:`claude-dev` workshop by name
and run the first agent
with the shared prompt from the repository root:

.. code-block:: console

   $ workshop launch claude-dev
   $ workshop run claude-dev -- run "Follow the instructions in @../prompts/rubik-shared.md"


The agent generates a complete Rubik's cube solver implementation.
When finished,
commit the results:

.. code-block:: console

   $ git add . && git commit -m "claude-code implementation"


Second worktree
~~~~~~~~~~~~~~~

Return to the parent directory
and create a worktree for the second agent:

.. code-block:: console

   $ cd ..
   $ git worktree add rubik-copilot
   $ cd rubik-copilot/


Launch the :program:`copilot-dev` workshop by name
and run the second agent with the same prompt:

.. code-block:: console

   $ workshop launch copilot-dev
   $ workshop run copilot-dev -- run "Follow the instructions in @../prompts/rubik-shared.md"


Commit the second implementation:

.. code-block:: console

   $ git add . && git commit -m "copilot-cli implementation"


Synthesis
~~~~~~~~~

Create a third worktree
where a synthesis agent will compare both implementations:

.. code-block:: console

   $ cd ..
   $ git worktree add rubik-synthesis
   $ cd rubik-synthesis/


Launch the :program:`claude-dev` workshop by name
and run the synthesis agent:

.. @artefact workshop run

.. code-block:: console

   $ workshop launch claude-dev
   $ workshop run claude-dev -- run "Follow the instructions in @../prompts/rubik-synthesis.md"


The synthesis agent walks through both implementations,
asking questions like:

.. code-block:: text

   Q: Implementation A uses a state-based solver with BFS,
      while Implementation B uses the Kociemba algorithm.
      Which solving approach do you prefer?

   A: option A

   Q: Implementation A structures the code in a single module,
      while Implementation B separates concerns across multiple modules.
      Which architecture do you prefer?

   A: option B


After gathering your preferences,
the agent creates the final synthesized implementation.


Verify the result
~~~~~~~~~~~~~~~~~

Test the synthesized Rubik's cube solver, for instance:

.. @artefact workshop exec

.. code-block:: console

   $ workshop exec claude-dev -- python rubik_solver.py


The 3D visualization should display
a Rubik's cube being solved.


Scenario 2: Role-based coding
-----------------------------

This scenario demonstrates a different workflow
where agents take on distinct roles:
one as an architect creating planning documents,
another as a coder implementing the design.


Design worktree
~~~~~~~~~~~~~~~

Start fresh from the parent directory:

.. code-block:: console

   $ cd /path/to/rubik-project
   $ git worktree add design
   $ cd design/


Launch the :program:`claude-dev` workshop by name
and run the architect agent with a role-specific model choice:

.. code-block:: console

   $ workshop launch claude-dev
   $ workshop run claude-dev -- run --model opus "Follow the instructions in @../prompts/rubik-architect.md"


The agent creates planning documents
in the current directory.
Commit these documents:

.. code-block:: console

   $ git add . && git commit -m "architecture and design docs"


Implementation worktree
~~~~~~~~~~~~~~~~~~~~~~~

Return to the parent directory
and create the implementation worktree:

.. code-block:: console

   $ cd ..
   $ git worktree add implementation
   $ cd implementation/


Launch the :program:`copilot-dev` workshop by name
and run the implementation agent:

.. code-block:: console

   $ workshop launch copilot-dev
   $ workshop run copilot-dev -- run "Follow the instructions in @../prompts/rubik-coder.md"


The coder agent reads the planning documents
from :file:`../design/`
without requiring file copying,
since all worktrees share the same parent directory structure.


Verify the result
~~~~~~~~~~~~~~~~~

Test the implementation:

.. @artefact workshop shell

.. code-block:: console

   $ workshop exec copilot-dev -- python rubik_solver.py


The 3D visualization should display
a working Rubik's cube solver.


Conclusion
----------

|ws_markup| provides a versatile environment
for development workflows that involve multiple complex tools such as AI agents.
The scenarios above demonstrate two popular patterns,
but real-world use cases could also include:

- Evals and benchmarks for side-by-side comparison

- Multi-layered role orchestration across many branches

- Additional personas like analyst, tester, or product owner,
  each with their own branch and agentic stack;
  extra capabilities such as skills or subagents can be used, too


By running agents in isolated worktrees
with a shared workshop sandbox,
you gain security and privacy benefits
while maintaining the flexibility
to experiment with different agent combinations and workflows.


See also
--------

Explanation:

- :ref:`exp_workshop_definition`
- :ref:`exp_workshop_definition_actions`


Reference:

- :ref:`ref_workshop_run`
- :ref:`ref_workshop_shell`
- :ref:`ref_workshop_exec`
- :ref:`ref_workshop_actions`
