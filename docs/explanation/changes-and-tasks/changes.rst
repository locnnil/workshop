Changes and Tasks
==================

*Change* is the core concept of the workspace state management system shared
with snapd. Every command (e.g. ``launch``) that changes the state of one or
multiple workspaces is internally expressed as a *Change*. The *Change*
comprises *Tasks* that executed in a predefined order. A task is a fairly small
and independent piece of logic. That can be mounting a project directory,
running an SDK hook or starting a workspace container. Most tasks contain an
undo logic which makes their progress reversible.

Thus, the state management system enables a granular control over the state of a
workspace container instance. An operation can be paused, continued or resumed
in case of an error.
