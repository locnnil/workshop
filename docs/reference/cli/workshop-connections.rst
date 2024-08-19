.. _ref_workshop_connections:

workshop connections
====================

Lists interface connections.

.. code-block:: console

   $ workshop connections [<WORKSHOP>] [OPTIONS]


Examples
--------

List connections for the workshop :samp:`nimble`
in the current project directory:

.. code-block:: console

   $ workshop connections nimble


List connections for all workshops in the current project directory:

.. code-block:: console

   $ workshop connections


Synopsis
--------

This command lists the connections between interface plugs and slots
for the entire project or a single workshop within it.
Each line represents a connection between a plug and a slot via an interface;
additional notes, including specific plug bindings, are provided as needed.


Notes
-----

- The output lists connections created with :ref:`ref_workshop_connect`
  as :samp:`manual`

- The :option:`!--all` option needn't be used with an argument;
  if a workshop is supplied, disconnected plugs are also listed


Options
-------

--all

  Include disconnected plugs in the output.


Global options
--------------

-h, --help

  Print the help message for the command.

-p, --project <DIRECTORY>

  Specify the project's directory path.


See also
--------

Explanation:

- :ref:`exp_interfaces`
- :ref:`exp_plugs_slots`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_disconnect`
