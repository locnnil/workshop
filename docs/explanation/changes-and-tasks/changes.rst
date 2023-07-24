Changes and Tasks
==================

*Change* is the core concept of the workspace state management system. Every
long-running or invasive (e.g. ``launch``) that changes the state of one or
multiple workspaces is planned and executed as a *Change*. The *Change*
comprises *Tasks* that executed in a predefined order. A task is a fairly small
and independent piece of logic. It can be mounting a project directory,
running an SDK hook or starting a workspace container. Most tasks contain an
undo logic which makes their progress reversible.

Thus, the state management system enables a granular control over the state of a
workspace container instance and prioritises the workspace integrity if
something does not follow a happy path. By default, any unsuccessfull change
reverts its progress to a previously working state.

Exploring changes and tasks
~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Consider the following scenario, where a project has two workspaces ``nimble``
and ``ml-transformer``. The latter uses an unstable SDK from the ``latest/edge``
channel as follows:

.. code-block:: yaml

    name: ml-transformer
    base: ubuntu@22.04
    sdks:
        huggingface:
            channel: latest/edge

Let's try to refresh both of the workspaces:

.. code-block:: bash

    $ workspace refresh nimble ml-transformer
    Error: cannot perform the following tasks:
    - Run hook "setup-base" for "huggingface" SDK (command failed with an error code (1))
    Refresh aborted

One can reason about the refresh failure by finding the the command's respective
change first:

.. code-block:: bash

    $ workspace changes
    ID  Status  Spawn                Ready                Summary
    81   Error   today at 12:20 NZST  today at 12:23 NZST  Refresh workspaces "nimble", "ml-transformer"

And inspecting its tasks and detailed logs:

.. code-block:: bash

    $ workspace tasks 81
    ID    Status  Spawn                Ready                Summary
    1383  Undone  today at 12:17 NZST  today at 12:18 NZST  Mount SDK state storage
    1384  Done    today at 12:17 NZST  today at 12:18 NZST  Run hook "save-state" for "go" SDK
    # ... the entire task list it ommited for brevity
    1389  Done    today at 12:17 NZST  today at 12:18 NZST  Retrieve "go" SDK from channel "latest/edge"
    1390  Undone  today at 12:17 NZST  today at 12:18 NZST  Install "go" SDK
    1391  Undone  today at 12:17 NZST  today at 12:18 NZST  Link "go" SDK
    1392  Error   today at 12:17 NZST  today at 12:18 NZST  Run hook "setup-base" for "go" SDK

    ......................................................................
    Run hook "save-state" for "go" SDK

    2023-07-24T12:17:37+12:00 INFO latest/beta save-state: preserving ~/.config/pretrained-config.conf
    ......................................................................
    Run hook "setup-base" for "go" SDK

    2023-07-24T12:18:06+12:00 INFO The edge version is not stable and not recommended for production use
    2023-07-24T12:18:06+12:00 ERROR command failed with an error code (1):
    Traceback (most recent call last):
        File "<string>", line 1, in <module>
        File "/home/user/.local/lib/python3.9/site-packages/tensorrt/__init__.py", line 36, in <module>
            from .tensorrt import *
    ModuleNotFoundError: No module named 'tensorrt.tensorrt'
