.. _workspace_launch:

workspace launch
================

Initialises one or many workspaces using their definitions.

.. code:: shell

   workspace launch <WORKSPACE>... [global options]


Synopsis
--------

This command constructs the workspaces listed as arguments by going over their
definitions and installing their components.  For each workspace, it:

- Checks the workspace definition and identifies necessary actions
- Retrieves the required components, such as base and SDKs
- Runs SDK setup hooks to initialise the working state
- On success, sets the workspace to *Ready*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are constructed.


Notes
-----

- Names listed as arguments must match
  respective :code:`name:` values in definitions
- To update an existing workspace, use :ref:`workspace_refresh` instead
- SDKs are installed in alphabetical order



Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`SDK (concept) <exp_sdk>`
- :ref:`workspace base (concept) <exp_workspace_base>`
- :ref:`workspace definition (concept) <exp_workspace_def>`

Reference:

- :ref:`workspace refresh (command) <workspace_refresh>`
