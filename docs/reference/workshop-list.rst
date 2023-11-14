.. _ref_workshop_list:

workshop list
=============

Lists project workshops.

.. code:: console

   $ workshop list [--global] [global options]


Synopsis
--------

This command enumerates all workshops in the project, printing a compact list:

- Project: absolute pathname of the project where this workshop belongs
- Workshop: workshop name, as set by its definition
- Status: workshop status, such as *Off*, *Ready*, *Pending* and so on
- Notes: internal remarks on the overall state of the workshop

The :option:`!--global` option
lists all workshops from *all* projects in the system;
however, it doesn't include any that are *Off*.


Notes
-----

- For details of a single workshop, use :ref:`ref_workshop_info` instead


Options
-------

--global

  List workshops from all projects in the system.


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`project (concept) <exp_project>`
- :ref:`workshop definition (concept) <exp_workshop_def>`

Reference:

- :ref:`workshop info (command) <ref_workshop_info>`
