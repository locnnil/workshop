Tutorial
========

This is a practical introduction
that takes you on a tour
of the essential |project| activities.


Install |project|
-----------------

Check the prerequisites,
build and install |project|,
then make sure it runs.


Check prerequisites
~~~~~~~~~~~~~~~~~~~

|project| requires
`LXD <https://ubuntu.com/lxd>`_
for low-level operation,
using its
`REST API <https://documentation.ubuntu.com/lxd/en/latest/restapi_landing/>`_
to configure individual *workshops*.

First, install and
`initialise <https://documentation.ubuntu.com/lxd/en/latest/howto/initialize/>`_
LXD.
It's available as a snap:

.. tabs::
   .. group-tab:: Using :program:`snap`

      .. code:: shell

         sudo snap install lxd
         sudo lxd init --auto


   .. group-tab:: Other ways

      See the available installation options in
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_.


Next, ensure the
`LXD daemon
<https://documentation.ubuntu.com/lxd/en/latest/explanation/lxd_lxc/#lxd-daemon>`_
is enabled and running:

.. tabs::
   .. group-tab:: Using :program:`snap`

      .. code:: shell

         sudo snap start --enable lxd.daemon
         snap services lxd.daemon

   .. group-tab:: Other ways

      Refer to
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_
      and your distribution's manuals for guidance.


Install
~~~~~~~

Build the ``workshop`` snap
from the |project| source code on
`GitHub
<https://github.com/canonical/workshop>`_:

.. code:: shell

   git clone git@github.com:canonical/workshop.git
   # -- or --
   git clone https://github.com/canonical/workshop.git
   cd workshop
   sudo snap install snapcraft --classic
   snapcraft

Install the resulting :file:`.snap` file,
for example:

.. code:: shell

   sudo snap install --devmode ./workshop_0.1.0_amd64.snap


Run
~~~

The snap installs two major components:

- The :program:`workshopd` daemon that exposes a REST API
- The :program:`workshop` CLI tool that uses this API to command |project|

The daemon starts automatically after installation;
the CLI tool is run manually:

.. code:: shell

   workshop --help


Launch a workshop
-----------------

Having installed |project|,
use it to define, launch, start and stop your first
:ref:`workshop <exp_workshop>`.


Define
~~~~~~

#. Create a
   :ref:`project directory <exp_project>`
   named :file:`hello-workshop`:

   .. code:: shell

      mkdir hello-workshop
      cd hello-workshop


#. In the project directory,
   create a
   :ref:`workshop definition <exp_workshop_def>`
   named :file:`.workshop.nimble.yaml`:

   .. code:: yaml

      name: nimble
      base: ubuntu@22.04
      sdks:
        go:
          channel: latest/stable


#. Make sure |project| can find the definition
   by *listing* the workshops
   in the project directory:

   .. code:: shell

      workshop list

          Project                Workshop   Status  Notes
          ~/hello-workshop       nimble     Off     -


   Note that a newly created workshop is *Off*.


Launch
~~~~~~

To prepare a workshop for action,
you :ref:`launch <ref_workshop_launch>` it:

.. code:: shell

   workshop launch nimble

       "nimble" launched


Now, the workshop is *Ready*
to build, debug and run code.

To make sure |project| watches the changes in the project directory,
move it, then run :command:`workshop list`:

.. code:: shell

   cd ..
   mv hello-workshop hi-workshop
   cd hi-workshop
   workshop list


       Project                Workshop   State  Notes
       ~/hi-workshop          nimble     Ready  -


Start and stop
~~~~~~~~~~~~~~

If you're done with the workshop for now,
*stop* it to conserve resources:

.. code:: shell

   workshop stop nimble

       "nimble" stopped


To resume, *start* the workshop again:

.. code:: shell

   workshop start nimble

       "nimble" started


Both commands operate gracefully,
waiting for the workshop to comply.


.. _tut_refresh:

Refresh a workshop
------------------

When an aspect of the workshop changes,
refresh it to pick up the update.


Update components
~~~~~~~~~~~~~~~~~

If the
:ref:`SDKs <exp_sdk>`
listed in the
:ref:`workshop definition <exp_workshop_def>`
are updated,
:ref:`refresh <ref_workshop_refresh>` the workshop to apply the changes:

.. code:: shell

   workshop refresh nimble

       "nimble" refreshed

The workshop is rebuilt from the
:ref:`base <exp_workshop_base>`;
then the SDKs are updated from their respective channels.

To refresh multiple workshops at once:

.. code:: shell

   workshop refresh nimble huggingface ...

.. note::

   The operation is transactional: If an error occurs,
   **all** changes in **all** listed workshops are reverted.


Add or remove an SDK
~~~~~~~~~~~~~~~~~~~~

To add a new SDK to your workshop,
update the definition file and refresh the workshop:

.. code:: yaml

   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
     huggingface:
       channel: latest/edge


.. code:: shell

   workshop refresh nimble

       "nimble" refreshed


To remove an SDK,
delete it from the definition and refresh the workshop.


.. _tut_refresh_wait_on_error:

Wait on error
~~~~~~~~~~~~~

To pause the refresh operation on error
instead of cancelling it outright,
add the :option:`!--wait-on-error` option:

.. code:: shell

   workshop refresh --wait-on-error nimble

       ERROR command failed with an error code (1): The edge version is not stable

       Error: "nimble" refresh failed, resolve all errors and run "workshop refresh --continue".
       To abort and get back to the state before run "workshop refresh --abort"

All progress is saved, up to the specific *task* that caused the error.
Then, you can explore the paused workshop
and choose to abort or continue the refresh operation.

To investigate the issue, check the recent
:ref:`changes <ref_workshop_changes>`:

.. code:: shell

   workshop changes

       ID  Status  Spawn                Ready                Summary
       ...
       81  Error   ...                  ...                  ...


Having found the problematic change, explore its
:ref:`tasks <ref_workshop_tasks>`:

   workshop tasks 81

       ...
       1391  Undone  today at 12:17       today at 12:18       Link "go" SDK
       1392  Error   today at 12:17       today at 12:18       Run hook "setup-base" for "go" SDK
       ...


.. note::

   For details, see :ref:`exp_changes_tasks`.


To continue the refresh operation:

.. code:: shell

    workshop refresh --continue nimble

        "nimble" refreshed


To abort the operation and recover the last operational state:

.. code:: shell

    workshop refresh --abort nimble

        "nimble" aborted


.. _tut_exec:

Execute commands
----------------

When the workshop is ready,
execute arbitrary commands in it using :ref:`ref_workshop_exec`:

.. code:: shell

   workshop exec nimble go build


To define environment variables and visibly separate the command's options:

.. code:: shell

   workshop exec nimble --env GOARCH=linux -- go build -x nimble.go


You can run an interactive shell as well:

.. code:: shell

   workshop exec nimble bash

   uname -a

       Linux nimble-bf3a1040 6.2.0-35-generic #35~22.04.1-Ubuntu SMP PREEMPT_DYNAMIC Fri Oct  6 10:23:26 UTC 2 x86_64 x86_64 x86_64 GNU/Linux


Persistent changes are saved in the project directory and the workshop itself.


.. _tut_remove:

Remove a workshop
-----------------

If you don't need a workshop anymore,
:ref:`remove <ref_workshop_remove>` it:

.. code:: shell

    workshop remove nimble

        "nimble" removed

This leaves the workshop definition intact.
