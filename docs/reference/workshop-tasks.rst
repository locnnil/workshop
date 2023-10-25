.. _ref_workshop_tasks:

workshop tasks
==============

Lists tasks for a specific change.

.. code:: shell

   workshop tasks <CHANGE ID> [global options]


Synopsis
--------

Any substantial operation on a workshop is a *change* that consists of *tasks*;
the command lists individual tasks that comprise a specific change.
For each task, it prints the following details:

- ID:      uniquely identifies the task within the change
- Status:  reflects the task's progress and affects the change's status
- Spawn:   tells when the task was started
- Ready:   tells when the task was finished
- Summary: lists actions, affected SDKs and workshops, other information


Notes
-----

- The command may print additional log details for tasks that store them
- To investigate recent changes in a project,
  use :ref:`ref_workshop_changes` instead


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`changes, tasks (concepts) <exp_changes_tasks>`
- :ref:`project (concept) <exp_project>`
- :ref:`workshop (concept) <exp_workshop>`

Reference:

- :ref:`workshop changes (command) <ref_workshop_changes>`
