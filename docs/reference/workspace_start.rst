.. _ref_workspace_start:

workspace start
===============

Starts one or many workspaces.

.. code:: shell

   workspace start <WORKSPACE>... [global options]


Synopsis
--------

This command activates the workspaces listed as arguments. For each one, it:

- Makes sure the workspace was actually launched
- Activates the workspace for use and sets it to *Ready*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are started.


Notes
-----

- If a workspace is already started or wasn't yet launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To stop a started workspace, use 'workspace stop'


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
- :ref:`workspace (concept) <exp_workspace>`

Reference:

- :ref:`workspace launch (command) <ref_workspace_launch>`
- :ref:`workspace stop (command) <ref_workspace_stop>`
