Launch a workspace
==================

Having installed |project|,
use it to define, launch, start and stop your first
:ref:`workspace <exp_workspace>`.


Define
------

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
------

To prepare a workspace for action,
you *launch* it:

.. code:: shell

   workspace launch nimble

       "nimble" launched


Now, the workspace is *Ready*
to build, debug and run code.

To make sure |project| watches the changes in the project directory,
move it and check the :command:`workspace list` output:

.. code:: shell

   cd ..
   mv hello-workspace hi-workspace
   cd hi-workspace
   workspace list


       Project                 Workspace  State  Notes
       ~/hi-workspace          nimble     Ready  -


Start and stop
--------------

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
