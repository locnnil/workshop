.. _ref_workshop_stop:

workshop stop
=============

Stops one or many workshops.

.. code:: console

   $ workshop stop <WORKSHOP>... [global options]


Synopsis
--------

This command deactivates the workshops listed as arguments. For each one, it:

- Makes sure the workshop was actually started or is already stopped
- Deactivates the workshop and sets it to *Stopped*

If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are stopped.

Notes
-----

- If a workshop wasn't yet started or even launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To start a stopped workshop, use 'workshop start'


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
- :ref:`workshop start (command) <ref_workshop_start>`
