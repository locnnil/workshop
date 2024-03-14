.. _ref_workshop_refresh:

workshop refresh
================

Updates workshops according to their definitions.

.. code-block:: console

   $ workshop refresh [--abort|--continue|--wait-on-error] <WORKSHOP>... [global options]


Synopsis
--------

This command updates the workshops listed as arguments by going over their
definitions once again.  For each workshop, it:

- Saves the working state of the workshop
- Checks the workshop definition and identifies any updates required
- Retrieves the updated components
- Applies and verifies the changes to the workshop
- Restores the working state of the workshop

The :option:`!--wait-on-error` option pauses the refresh if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workshop.
If multiple workshops are listed and an error occurs,
the operation is aborted and reverted for *all* of them.


Notes
-----

- The workshop must be *Ready* to be refreshed
- To construct a newly defined workshop,
  use :ref:`ref_workshop_launch` instead
- Throughout the refresh, all affected workshops remain *Pending*
- If the refresh removes an SDK from the workshop, the SDK state isn't saved
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


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`exp_project`
- :ref:`exp_workshop`
- :ref:`exp_workshop_def`

Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_remove`
