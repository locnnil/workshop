.. _how_debug_issues_workshops:

How to debug issues in workshops
================================

To trace the condition of a misbehaving workshop,
you can explore its underlying changes and tasks,
list system-wide warnings and acknowledge false positives.
This may help identify the root cause
if a :command:`refresh` or any other action fails.


List workshop changes
---------------------

Consider a workshop named :samp:`golang-volatile`
that uses an unstable SDK
from the :samp:`latest/edge` channel:

.. code-block:: yaml
   :caption: .workshop.golang-volatile.yaml

   name: golang-volatile
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/edge


Suppose something goes during a :command:`refresh`:

.. code-block:: console

   $ workshop refresh golang-volatile

        Error: cannot perform the following tasks:
        - Run hook "setup-base" for "go" SDK (command failed with an error code (1))
        Refresh aborted


To investigate the failure,
list the *changes* in the workshop to find the one that failed:

.. code-block:: console

   $ workshop changes

       ID  Status  Spawn                Ready                Summary
       ...
       81  Error   today at 12:20       today at 12:23       Refresh workshops "golang-volatile"


List tasks in a change
----------------------

When the problematic change is found,
list its *tasks* to see the cause:

.. code-block:: console

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


List and suppress warnings
--------------------------

Occasionally, |project_markup| encounters non-blocking or transient issues,
such as broken mount points.
As such, they are registered as *warnings* in a system-wide log
that can be accessed with :command:`workshop warnings`:

.. code-block:: console

   $ workshop warnings

       last-occurrence:  4 days ago, at 17:52 GMT
       warning: |
         golang-volatile/go:mod-cache mount is broken: /home/user/mod-cache does not exist


Multiple warnings reporting one issue aren't stacked;
only their first and last occurrences are recorded.
You can suppress listed warnings with :command:`workshop okay` to ignore them:

.. code-block:: console

   $ workshop okay


See also
--------

Explanation:

- :ref:`exp_changes_tasks`
- :ref:`exp_sdk`
- :ref:`exp_workshop`


Reference:

- :ref:`ref_workshop_changes`
- :ref:`ref_workshop_okay`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_tasks`
- :ref:`ref_workshop_warnings`
