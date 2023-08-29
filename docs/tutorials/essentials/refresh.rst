.. _tut_refresh:

Refresh a workspace
===================

When an aspect of the workspace changes,
refresh it to pick up the update.


Update versions
---------------

If SDKs listed in the definition file are updated,
*refresh* the workspace to apply the updates:

.. code-block:: bash

   workspace refresh nimble

       "nimble" refreshed

The workspace is rebuilt from the base;
then the SDKs are updated from their respective channels.
If your project has multiple workspaces,
``workspace refresh`` simultaneously updates all of them.

.. note::

   The operation is transactional: If an error occurs,
   **all** changes in **all** affected workspaces are reverted.


Add or remove an SDK
--------------------

To add a new SDK to your workspace,
update the definition file and **refresh** the workspace:

.. code-block:: yaml

   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
     huggingface:
       channel: latest/edge


.. code-block:: bash

   workspace refresh nimble

       "nimble" refreshed


To remove an SDK,
delete it from the definition and refresh the workspace.


Wait on error
-------------

To pause the refresh operation on error
instead of canceling it outright,
add the ``--wait-on-error`` option:

.. code-block:: bash

   workspace refresh --wait-on-error nimble

       ERROR command failed with an error code (1): The edge version is not stable

       Error: "nimble" refresh failed, resolve all errors and run "workspace refresh --continue".
       To abort and get back to the state before run "workspace refresh --abort"

All progress is saved, up to the specific *task* that caused the error.
Then, you can explore the paused workspace
and choose to abort or continue the refresh operation.

To investigate the issue, check the recent *changes and tasks*:

.. code-block:: bash

   workspace changes

       ID  Status  Spawn                Ready                Summary
       ...
       81  Error   ...                  ...                  ...

   workspace tasks 81

       ...
       1391  Undone  today at 12:17       today at 12:18       Link "go" SDK
       1392  Error   today at 12:17       today at 12:18       Run hook "setup-base" for "go" SDK
       ...


.. note::

   For details, see :ref:`tut_changes_tasks`.


To continue the refresh operation:

.. code-block:: bash

    workspace refresh --continue nimble

        "nimble" refreshed


To abort and recover the last operational state:

.. code-block:: bash

    workspace refresh --abort nimble

        "nimble" aborted
