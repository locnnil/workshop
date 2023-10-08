.. _ref_workspace_list:

workspace list
==============

Lists project workspaces.

.. code:: shell

   workspace list [--global] [global options]


Synopsis
--------

This command enumerates all workspaces in the project, printing a compact list:

- Project: absolute pathname of the project where this workspace belongs
- Workspace: workspace name, as set by its definition
- State: workspace status, such as *Off*, *Ready*, *Pending* and so on
- Notes: internal remarks on the overall state of the workspace

The :option:`!--global` option
lists all workspaces from *all* projects in the system;
however, it doesn't include any that are *Off*.


Notes
-----

- For details of a single workspace, use :ref:`ref_workspace_info` instead


Options
-------

--global

  List workspaces from all projects in the system.


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`project (concept) <exp_project>`
- :ref:`workspace definition (concept) <exp_workspace_def>`

Reference:

- :ref:`workspace info (command) <ref_workspace_info>`
