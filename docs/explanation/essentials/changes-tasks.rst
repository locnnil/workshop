:hide-toc:

.. _exp_changes_tasks:

Changes, tasks
==============

A *change* is the core concept of the workshop state management system.
Any long-running or invasive operation
(e.g. :ref:`launch <ref_workshop_launch>`)
that changes the state of a workshop
is planned and applied as a change,
which comprises specific tasks
that run in a predefined order.

A *task* is a small, independent piece of logic;
it can be mounting a project directory,
running a :ref:`hook <exp_sdk_hooks>`
or starting a workshop container.
Most tasks are reversible.

Overall, this scheme enables granular control
over the state of a workshop;
the state management system uses it
to ensure the integrity of the workshop on errors.
By default, a failed change reverts the workshop
to the last operational state.


See also
--------

Explanation:

- :ref:`exp_sdk_hooks`


How-to guides:

- :ref:`how_debug_issues_workshops`


Reference:

- :ref:`ref_workshop_changes`
- :ref:`ref_workshop_launch`


Tutorial:

- :ref:`tut_refresh_wait_on_error`
