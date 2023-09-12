.. _workspace_refresh:

workspace refresh
=================

Updates workspaces according to their definitions.

.. code:: shell

   workspace refresh [--abort|--continue|--wait-on-error] <WORKSPACE>... [flags]


Synopsis
--------

This command updates the workspaces listed as arguments by going over their
definitions once again.  For each workspace, it:

- Saves the working state of the workspace
- Checks the workspace definition and identifies any updates required
- Retrieves the updated components
- Applies and verifies the changes to the workspace
- Restores the working state of the workspace

The :option:`!--wait-on-error` option pauses the refresh if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workspace.
If multiple workspaces are listed and an error occurs,
the operation is aborted and reverted for *all* of them.


Notes
-----

- The workspace must be *Ready* to be refreshed
- Throughout the refresh, all affected workspaces remain *Pending*
- If the refresh removes an SDK from the workspace, the SDK state isn't saved
- Updated and newly added SDKs are installed in alphabetical order


Options
-------

--abort

  Abort the previously paused operation, reverting any changes.

--continue

  Continue the previously paused operation.

--wait-on-error

  Pause the operation on error; to resume, use :option:`!--continue`
  or :option:`!--abort`.


Options inherited from parent commands
--------------------------------------

-p, --project <DIRECTORY>

  Specify a project's directory path.


See also
--------

Explanations: :ref:`workspace (concept) <exp_workspace>`
