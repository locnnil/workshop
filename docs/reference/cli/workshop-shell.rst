.. _ref_workshop_shell:

workshop shell
==============

Starts an interactive terminal session for the workshop.

.. code-block:: console

   $ workshop shell <WORKSHOP> [global options]


Synopsis
--------

The :program:`shell` subcommand runs an interactive terminal session
in the specified workshop.

To accept a :program:`shell` command,
the workshop must be *Ready* or *Pending*.


Notes
-----

- To start a workshop before running a terminal session,
  use :ref:`ref_workshop_start`
- The subcommand is a shorthand for :ref:`ref_workshop_exec`;
  it launches the login shell for :samp:`workshop`,
  the default non-privileged user in a workshop


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

- :ref:`workshop exec (command) <ref_workshop_exec>`
- :ref:`workshop start (command) <ref_workshop_start>`