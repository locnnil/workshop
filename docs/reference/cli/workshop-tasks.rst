.. _ref_workshop_tasks:

workshop tasks
==============

Lists tasks for a specific change.

.. code-block:: console

   $ workshop tasks <CHANGE ID> [OPTIONS]


Examples
--------

List the tasks under change ID :samp:`42`:

.. code-block:: console

   $ workshop tasks 42


Synopsis
--------

Any substantial operation on a workshop is a *change* that consists of *tasks*;
the command lists individual tasks that comprise a specific change.
For each task, it prints the following details:

+---------+----------------------------------------------------------------+
| ID      | Uniquely identifies the task within the change                 |
+---------+----------------------------------------------------------------+
| Status  | Reflects the task's progress and affects the change's status   |
+---------+----------------------------------------------------------------+
| Spawn   | Tells when the task was started                                |
+---------+----------------------------------------------------------------+
| Ready   | Tells when the task was finished                               |
+---------+----------------------------------------------------------------+
| Summary | Lists actions, affected SDKs and workshops, other information  |
+---------+----------------------------------------------------------------+


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

- :ref:`exp_changes_tasks`
- :ref:`exp_projects`
- :ref:`exp_workshop`

Reference:

- :ref:`ref_workshop_changes`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_list`
