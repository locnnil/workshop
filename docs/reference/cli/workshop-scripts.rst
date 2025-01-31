.. _ref_workshop_scripts:

workshop scripts
----------------

List workshop scripts.

.. rubric:: Usage

.. code-block:: console

   $ workshop scripts [flags]

.. rubric:: Description


This command enumerates all scripts in the workshop, printing a YAML map.


.. rubric:: Examples


List scripts for the 'nimble' workshop in the current project directory:

.. code-block:: console

   $ workshop scripts nimble


The name is optional if the project has only one workshop:

.. code-block:: console

   $ workshop scripts


