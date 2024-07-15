.. _ref_workshop_okay:

workshop okay
=============

Acknowledges listed warnings.

.. code-block:: console

   $ workshop okay [OPTIONS]


Examples
--------

Acknowledge the globally registered warnings across all workshops
(must run after :ref:`ref_workshop_warnings`):

.. code-block:: console

   $ workshop okay


Synopsis
--------

This command acknowledges all warnings
listed previously by the :ref:`ref_workshop_warnings` command.


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Reference:

- :ref:`ref_workshop_changes`
- :ref:`ref_workshop_tasks`
- :ref:`ref_workshop_warnings`
