Iterate on a workspace
======================

The workspace file of an actual project may contain multiple SDKs, interfaces,
packages and life cycle hooks. When approached for the first time, it is likely
that designing a workspace will take a few iterations before arriving at the
desired development environment for your project.

.. note::
    It is highly recommended to familiarise yourself with the concept :ref:`changes-and-tasks`
    before proceeding. The workspace state engine gives a fine control over how a
    long-running or invasive operation will be planned and executed by prioritising
    always having a workspace in a working state.

With the ``wait-on-error`` option, the refresh command will not initiate  abort
automatically. Instead, the progress will be paused on the task that caused an
error. It makes debugging the workspace issues faster by exploring the workspace
at the exact point of failure and aborting or continuing the operation
afterwards:

.. code-block:: bash

    $ workspace refresh --wait-on-error nimble
    2023-07-24T14:10:33+12:00 ERROR command failed with an error code (1): The edge version is not stable

    Error: "nimble" refresh failed, resolve all errors and run "workspace refresh --continue".
    To abort and get back to the state before run "workspace refresh --abort"

Investigate the issue by exploring the tasks statuses and logs:

.. code-block:: bash

    $ workspace changes
    # ...
    $ workspace tasks 1
    # ...
    1391  Undone  today at 12:17 NZST  today at 12:18 NZST  Link "go" SDK
    1392  Error   today at 12:17 NZST  today at 12:18 NZST  Run hook "setup-base" for "go" SDK
    # ...

Then either continue the refresh operation:

.. code-block:: bash

    $ workspace refresh --continue nimble
    "nimble" refreshed

...or abort and start from the latest working state:

.. code-block:: bash

    $ workspace refresh --abort nimble
    "nimble" aborted
