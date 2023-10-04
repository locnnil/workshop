:hide-toc:

.. _exp_changes_tasks:

Changes, tasks
==============

A *change* is the core concept of the workspace state management system.
Any long-running or invasive operation
(e.g. :ref:`launch <ref_workspace_launch>`)
that changes the state of a workspace
is planned and applied as a change,
which comprises specific tasks
that run in a predefined order.

A *task* is a small, independent piece of logic;
it can be mounting a project directory,
running a life cycle hook
or starting a workspace container.
Most tasks are reversible.

Overall, this scheme enables granular control
over the state of a workspace;
the state management system uses it
to ensure the integrity of the workspace on errors.
By default, a failed change reverts the workspace
to the last operational state.

See also
--------

How-to guides: :ref:`how_debug_workspace_issues`

Tutorial: :ref:`tut_refresh_wait_on_error`
