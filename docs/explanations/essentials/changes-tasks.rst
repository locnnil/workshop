.. _exp_changes_tasks:

Changes, tasks
==============

*Change* is the core concept of the workspace state management system. Every
long-running or invasive operation (e.g. ``launch``) that changes the state of
one or multiple workspaces is planned and executed as a *Change*. The *Change*
comprises *Tasks* that executed in a predefined order. A task is a fairly small
and independent piece of logic. It can be mounting a project directory, running
an SDK hook or starting a workspace container. Most tasks contain an undo logic
which makes their progress reversible.

Thus, the state management system enables a granular control over the state of a
workspace container instance and prioritises the workspace integrity if
something does not follow a happy path. By default, any unsuccessful change
reverts its progress to a previously working state.

The workspace state engine gives a fine control over how a long-running or
invasive operation will be planned and executed by prioritising always having a
workspace in a working state.
