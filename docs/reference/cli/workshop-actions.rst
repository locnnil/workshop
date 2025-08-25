.. _ref_workshop_actions:

workshop actions
----------------

.. @artefact workshop actions

List workshop actions.

.. rubric:: Usage

.. code-block:: console

   $ workshop actions [<WORKSHOP>] [flags]

.. rubric:: Description


This command enumerates all actions in the workshop, printing a YAML map.


.. rubric:: Examples


List actions for the 'nimble' workshop in the current project directory:

.. code-block:: console

   $ workshop actions nimble


The name is optional if the project has only one workshop:

.. code-block:: console

   $ workshop actions




