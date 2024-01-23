.. _ref_workshop_start:

workshop start
==============

Starts one or many workshops.

.. code-block:: console

   $ workshop start <WORKSHOP>... [global options]


Synopsis
--------

This command activates the workshops listed as arguments. For each one, it:

- Makes sure the workshop was actually launched
- Activates the workshop for use and sets it to *Ready*

If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are started.


Notes
-----

- If a workshop is already started or wasn't yet launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To stop a started workshop, use :ref:`ref_workshop_stop`


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
- :ref:`workshop (concept) <exp_workshop>`


Reference:

- :ref:`workshop launch (command) <ref_workshop_launch>`
- :ref:`workshop stop (command) <ref_workshop_stop>`
