.. _ref_workshop_shell:

workshop shell
==============

Starts an interactive terminal session for the workshop.

.. code-block:: console

   $ workshop shell <WORKSHOP> [OPTIONS]


Examples
--------

Open the default login shell of the :samp:`workshop` user
into the :samp:`nimble` workshop
in the current project directory:

.. code-block:: console

   $ workshop shell nimble


Synopsis
--------

The :samp:`shell` subcommand runs an interactive terminal session
in the specified workshop.

To accept a :samp:`shell` command,
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

- :ref:`exp_projects`
- :ref:`exp_workshop`


Reference:

- :ref:`ref_workshop_exec`
- :ref:`ref_workshop_start`
