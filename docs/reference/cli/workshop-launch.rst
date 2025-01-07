.. _ref_workshop_launch:

workshop launch
---------------

Construct one or many workshops using their definitions.

.. rubric:: Synopsis

.. code-block:: console

   $ workshop launch [--abort|--continue|--wait-on-error] <WORKSHOP>... [flags]

.. rubric:: Description


This command constructs the workshops listed as arguments by going over their
definitions and installing their components. For each workshop, it:

- Checks the workshop definition and identifies necessary actions

- Retrieves the required components, such as base and SDKs

- Runs SDK setup hooks to initialise the working state

- On success, ties the workshop to the project and starts it

The '--wait-on-error' option pauses the launch if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workshop.
If multiple workshops are listed and an error occurs,
the operation is aborted and reverted for all of them.

Notes:

- Names listed as arguments must match respective 'name:' values in definitions

- To update an existing workshop, use 'workshop refresh' instead

- SDKs are installed in alphabetical order


.. rubric:: Options


--abort

   Abort the previously paused operation, reverting any changes.


--continue

   Continue the previously paused operation.


--wait-on-error

   Pause the operation on error; to resume, use '--continue' or '--abort'.


--no-wait

   Return the change ID, don't wait for the operation to finish



.. rubric:: Examples


Launch the 'nimble' and 'jazzy' workshops in the current project directory:

.. code-block:: console

   $ workshop launch nimble jazzy


The name is optional if the project has only one workshop:

.. code-block:: console

   $ workshop launch


Launch 'nimble', but stop on any errors (won’t accept multiple workshops):

.. code-block:: console

   $ workshop launch nimble --wait-on-error


After 'nimble' launch stopped on error, abort the operation:

.. code-block:: console

   $ workshop launch nimble --abort


After 'nimble' launch stopped on error and the workshop was fixed,
continue the operation:

.. code-block:: console

   $ workshop launch nimble --continue
