.. _ref_ai_agents:

.. meta::
   :description: Reference for Workshop's AI-agent integration points,
                 listing the LLM-readable documentation URLs and the
                 use-workshop and sdk-designer agentic skills.

Workshop and AI agents
======================

.. @artefact SDK
.. @artefact workshop (container)

|ws_markup| integrates with AI coding agents,
exposing documentation as Markdown that agents can fetch and parse directly,
and agentic skills that wrap |ws_markup| and |sdk_markup| operations
so agents don't have to rediscover the CLIs every session.


.. _ref_ai_discovery:

LLM-readable docs
-----------------

..
   |ws_markup| publishes two files that follow the
.. `llms.txt convention <https://llmstxt.org/>`_:
.. `llms.txt <https://ubuntu.com/workshop/docs/llms.txt>`_
.. indexes every page with a one-line summary,
.. and `llms-full.txt <https://ubuntu.com/workshop/docs/llms-full.txt>`_
.. concatenates every page as Markdown.

To fetch a single page as Markdown,
append :file:`.md` to its URL.
For example,
this page is available at
:samp:`https://ubuntu.com/workshop/docs/reference/ai-agents.md`.


.. _ref_ai_use_workshop_skill:

The use-workshop skill
----------------------

The `use-workshop-skill <https://github.com/canonical/use-workshop-skill>`_ repository
ships an agentic skill for operating the |ws_markup| CLI:
launching workshops,
refreshing them,
running commands inside,
wiring interfaces,
debugging failed changes,
and orchestrating parallel environments via Git worktrees.

To enable it in a repository,
copy :file:`.github/skills/use-workshop/` into the target repo,
using the skills path for your agent
(:file:`.claude/skills/` for Claude Code,
:file:`.github/skills/` for Copilot).
Mention |ws_markup| in any prompt to trigger the skill.


.. _ref_ai_sdk_designer_skill:

The sdk-designer skill
----------------------

The `template-sdk <https://github.com/canonical/template-sdk>`_ repository
ships an agentic skill named :samp:`sdk-designer`.
The skill runs an interactive scaffolding conversation:
it asks about the software to package,
the target platforms,
and which interfaces and hooks are needed,
then writes the corresponding files into the template.

#. Aim the agent at the new repository.

#. Run :samp:`/sdk-designer` and answer the prompts.

#. Review the generated files
   and adjust where the skill's defaults don't match your case.


See also
--------

How-to guides:

- :ref:`how_use_workshops_with_ai_agents`
