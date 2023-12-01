.. _how_debug_issues_workshops:

How to debug issues in workshops
================================

To trace the condition of a misbehaving
:ref:`workshop <exp_workshop>`,
you can explore its underlying
:ref:`changes and tasks <exp_changes_tasks>`.
This may help identify the root cause
if a
:ref:`refresh <tut_refresh>`
fails.


List workshop changes
---------------------

Consider a workshop named ``golang-volatile``
that uses an unstable
:ref:`SDK <exp_sdk>`
from the ``latest/edge`` channel:

.. code:: yaml

   name: golang-volatile
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/edge


Suppose something goes during a
:ref:`refresh <tut_refresh>`
operation:

.. code:: console

   $ workshop refresh golang-volatile

        Error: cannot perform the following tasks:
        - Run hook "setup-base" for "go" SDK (command failed with an error code (1))
        Refresh aborted


To investigate the failure,
list the *changes* in the workshop to find the one that failed:

.. code:: console

   $ workshop changes

       ID  Status  Spawn                Ready                Summary
       ...
       81  Error   today at 12:20       today at 12:23       Refresh workshops "golang-volatile"


List tasks in a change
----------------------

When the problematic change is found,
list its *tasks* to see the cause:

.. code:: console

   $ workshop tasks 81

       ID    Status  Spawn                Ready                Summary
       ...
       1392  Error   today at 12:17       today at 12:18       Run hook "setup-base" for "go" SDK

       ......................................................................
       Run hook "save-state" for "go" SDK

       2023-07-24T12:17:37+12:00 INFO latest/beta save-state: preserving ~/.config/pretrained-config.conf
       ......................................................................
       Run hook "setup-base" for "go" SDK
       ...
       Traceback (most recent call last):
           File "<string>", line 1, in <module>
           File "/home/user/.local/lib/python3.9/site-packages/tensorrt/__init__.py", line 36, in <module>
               from .tensorrt import *
       ModuleNotFoundError: No module named 'tensorrt.tensorrt'

The SDK-specific reason can be addressed individually.
