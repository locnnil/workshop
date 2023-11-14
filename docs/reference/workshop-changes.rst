.. _ref_workshop_changes:

workshop changes
================

Lists recent changes to the workshops in a project.

.. code:: console

   $ workshop changes [global options]


Synopsis
--------

Any substantial operation on a workshop is a *change* that consists of *tasks*;
the command lists details of recent changes for all workshops within a project.
For each change, it prints the following details:

- ID:      uniquely identifies the change within the project
- Status:  reflects the change's progress and affects the workshop's status
- Spawn:   tells when the change was started
- Ready:   tells when the change was *successfully* finished, if at all
- Summary: lists actions, affected workshops, other information


Notes
-----

- Only successful changes display values in the *Ready* column
- To investigate the details of a specific change,
  use :ref:`ref_workshop_tasks` instead


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

- :ref:`workshop tasks (command) <ref_workshop_tasks>`
