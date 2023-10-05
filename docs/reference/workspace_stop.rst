.. _ref_workspace_stop:

workspace stop
==============

Stops one or many workspaces.

.. code:: shell

   workspace stop <WORKSPACE>... [global options]


Synopsis
--------

This command deactivates the workspaces listed as arguments. For each one, it:

- Makes sure the workspace was actually started or is already stopped
- Deactivates the workspace and sets it to *Stopped*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are stopped.

Notes
-----

- If a workspace wasn't yet started or even launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To start a stopped workspace, use 'workspace start'


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
- :ref:`workspace start (command) <ref_workspace_start>`
