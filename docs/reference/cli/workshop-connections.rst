.. _ref_workshop_connections:

workshop connections
====================

Lists interface connections.

.. code-block:: console

   $ workshop connect <WORKSHOP>/<SDK>:<PLUG> [<WORKSHOP>/<SDK>][:<SLOT>] [OPTIONS]


Synopsis
--------

This command lists connections between interface plugs and slots
for the entire project or for a single workshop in it;
each line represents a connection between a plug and a slot via an interface,
with extra notes provided if needed.


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
