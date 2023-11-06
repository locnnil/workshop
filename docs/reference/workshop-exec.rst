.. _ref_workshop_exec:

workshop exec
=============

Runs a command and waits for it to complete.

.. code:: shell

   workshop exec <WORKSHOP> [-i|-I] [--timeout <TIME>] [-w <DIR>] [-uid <USER>] [-gid <GROUP>] <COMMAND>


Synopsis
--------

The :program:`exec` subcommand runs an arbitrary command
in the specified workshop, waiting for it to complete.
If a timeout elapses before that, it's terminated.

To accept an :program:`exec` command,
the workshop must be *Ready* or *Pending*.
A command can run in two modes that determine how it handles standard streams:

- Interactively (for shell sessions)
- Non-interactively (for scripts)

To set the mode explicitly, use :option:`!-i` or :option:`!-I`.
If neither is supplied,
:program:`exec` deduces the mode based on the nature of its own streams:

- If :samp:`stdin` and :samp:`stdout` are terminals, the mode is interactive
- Otherwise, it's non-interactive

To separate the :program:`exec` subcommand from the command itself,
use shell syntax such as :samp:`--`:

.. code:: shell

   workshop exec nimble -- echo -n foo bar


Notes
-----

- To start a workshop before running commands in it,
  use :ref:`ref_workshop_start`
- You can set the working directory, environment variables, user and group ID
  for running the command in the workshop; reasonable defaults are provided


Options
-------

-w, --cwd <DIRECTORY>

  Set the working directory in the workshop (default: :file:`/project/`).

--env <KEY=VALUE>

  Set an environment variable.

--uid <USER ID>

  Run as a specific workshop user
  (default: :samp:`1000`/:samp:`workshop`).

--gid <GROUP ID>

  Run as a member of a specific workshop group
  (default: :samp:`1000`/:samp:`workshop`).

--timeout <DURATION>

  Set a timeout; valid units are :samp:`ns`, :samp:`us` or :samp:`µs`,
  :samp:`ms`, :samp:`s`, :samp:`m`, :samp:`h`.

-i, --interactive

  Force interactive mode.

-I, --non-interactive

  Force non-interactive mode.


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

- :ref:`workshop start (command) <ref_workshop_start>`
