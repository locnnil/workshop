Your first workspace
====================

When |project| is installed,
you're ready to define, launch, start and stop your first *workspace*.


Define
------

#. Create a project directory named ``hello-workspace``:

   .. code-block:: bash

      mkdir hello-workspace
      cd hello-workspace


#. In the project directory,
   create a workspace file named ``.workspace.nimble.yaml``:

   .. code-block:: yaml

      name: nimble
      base: ubuntu@22.04
      sdks:
        go:
          channel: latest/stable


#. Make sure |project| can find the definition
   by *listing* the workspaces
   in the project directory:

   .. code-block:: bash

      workspace list

          Project                 Workspace  State  Notes
          ~/hello-workspace       nimble     Off    -


   Note that a newly created workspace is *Off*.


Launch
------

To prepare a workspace for action,
you *launch* it:

.. code-block:: bash

   workspace launch nimble

       "nimble" launched


Now, the workspace is *Ready*
to build, debug and run code.


Start and stop
--------------

If you're done with the workspace for now,
*stop* it to conserve resources:

.. code-block:: bash

   workspace stop nimble

       "nimble" stopped


To resume, *start* the workspace again:

.. code-block:: bash

   workspace start nimble

       "nimble" started


Both commands operate gracefully,
waiting for the workspace to comply.
