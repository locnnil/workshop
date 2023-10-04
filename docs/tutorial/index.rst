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
to configure individual *workspaces*.

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

Build the ``workspace`` snap
from the |project| source code on
`GitHub
<https://github.com/canonical/workspace>`_:

.. code:: shell

   git clone git@github.com:canonical/workspace.git
   # -- or --
   git clone https://github.com/canonical/workspace.git
   cd workspace
   sudo snap install snapcraft --classic
   snapcraft

Install the resulting :file:`.snap` file,
for example:

.. code:: shell

   sudo snap install --devmode ./workspace_0.1.0_amd64.snap


Run
~~~

The snap installs two major components:

- The :program:`workspaced` daemon that exposes a REST API
- The :program:`workspace` CLI tool that uses this API to command |project|

The daemon starts automatically after installation;
the CLI tool is run manually:

.. code:: shell

   workspace --help


Launch a workspace
------------------

Having installed |project|,
use it to define, launch, start and stop your first
:ref:`workspace <exp_workspace>`.


Define
~~~~~~

#. Create a
   :ref:`project directory <exp_project>`
   named :file:`hello-workspace`:

   .. code:: shell

      mkdir hello-workspace
      cd hello-workspace


#. In the project directory,
   create a
   :ref:`workspace definition <exp_workspace_def>`
   named :file:`.workspace.nimble.yaml`:

   .. code:: yaml

      name: nimble
      base: ubuntu@22.04
      sdks:
        go:
          channel: latest/stable


#. Make sure |project| can find the definition
   by *listing* the workspaces
   in the project directory:

   .. code:: shell

      workspace list

          Project                 Workspace  State  Notes
          ~/hello-workspace       nimble     Off    -


   Note that a newly created workspace is *Off*.


Launch
~~~~~~

To prepare a workspace for action,
you :ref:`launch <ref_workspace_launch>` it:

.. code:: shell

   workspace launch nimble

       "nimble" launched


Now, the workspace is *Ready*
to build, debug and run code.

To make sure |project| watches the changes in the project directory,
move it, then run :command:`workspace list`:

.. code:: shell

   cd ..
   mv hello-workspace hi-workspace
   cd hi-workspace
   workspace list


       Project                 Workspace  State  Notes
       ~/hi-workspace          nimble     Ready  -


Start and stop
~~~~~~~~~~~~~~

If you're done with the workspace for now,
*stop* it to conserve resources:

.. code:: shell

   workspace stop nimble

       "nimble" stopped


To resume, *start* the workspace again:

.. code:: shell

   workspace start nimble

       "nimble" started


Both commands operate gracefully,
waiting for the workspace to comply.


.. _tut_refresh:

Refresh a workspace
-------------------

When an aspect of the workspace changes,
refresh it to pick up the update.


Update components
~~~~~~~~~~~~~~~~~

If the
:ref:`SDKs <exp_sdk>`
listed in the
:ref:`workspace definition <exp_workspace_def>`
are updated,
:ref:`refresh <ref_workspace_refresh>` the workspace to apply the changes:

.. code:: shell

   workspace refresh nimble

       "nimble" refreshed

The workspace is rebuilt from the
:ref:`base <exp_workspace_base>`;
then the SDKs are updated from their respective channels.

To refresh multiple workspaces at once:

.. code:: shell

   workspace refresh nimble huggingface ...

.. note::

   The operation is transactional: If an error occurs,
   **all** changes in **all** listed workspaces are reverted.


Add or remove an SDK
~~~~~~~~~~~~~~~~~~~~

To add a new SDK to your workspace,
update the definition file and refresh the workspace:

.. code:: yaml

   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
     huggingface:
       channel: latest/edge


.. code:: shell

   workspace refresh nimble

       "nimble" refreshed


To remove an SDK,
delete it from the definition and refresh the workspace.


.. _tut_refresh_wait_on_error:

Wait on error
~~~~~~~~~~~~~

To pause the refresh operation on error
instead of cancelling it outright,
add the :option:`!--wait-on-error` option:

.. code:: shell

   workspace refresh --wait-on-error nimble

       ERROR command failed with an error code (1): The edge version is not stable

       Error: "nimble" refresh failed, resolve all errors and run "workspace refresh --continue".
       To abort and get back to the state before run "workspace refresh --abort"

All progress is saved, up to the specific *task* that caused the error.
Then, you can explore the paused workspace
and choose to abort or continue the refresh operation.

To investigate the issue, check the recent *changes and tasks*:

.. code:: shell

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

   For details, see :ref:`exp_changes_tasks`.


To continue the refresh operation:

.. code:: shell

    workspace refresh --continue nimble

        "nimble" refreshed


To abort the operation and recover the last operational state:

.. code:: shell

    workspace refresh --abort nimble

        "nimble" aborted
