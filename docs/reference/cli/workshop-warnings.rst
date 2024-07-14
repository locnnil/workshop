.. _ref_workshop_warnings:

workshop warnings
=================

Lists warnings.

.. code-block:: console

   $ workshop warnings [OPTIONS]


Examples
--------

List the globally registered warnings across all workshops:

.. code-block:: console

   $ workshop warnings


Synopsis
--------

This command lists the warnings that were reported to the system.

All warnings listed by :command:`workshop warnings`
can be acknowledged with the :ref:`ref_workshop_okay` command.
Acknowledged warnings aren't listed by :command:`workshop warnings`
unless they occur again after their cooldown period has elapsed
or the :option:`!--all` option is used.

Also, warnings expire automatically; expired warnings are not listed.


Options
-------

--abs-time

  Use absolute times in RFC 3339 format.
  By default, relative times are used up to 60 days, then YYYY-MM-DD.

--all

  Show all warnings, including the acknowledged ones

--unicode [auto|never|always]

  Use Unicode characters to improve legibility.
  By default, Unicode is used only if the output supports it (:samp:`auto`).

--verbose

  Show more information per each warning.


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
- :ref:`ref_workshop_okay`
- :ref:`ref_workshop_tasks`
