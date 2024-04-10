.. _ref_workshop_connect:

workshop connect
================

Connects a plug to a slot.

.. code-block:: console

   $ workshop connect <WORKSHOP>/<SDK>:<PLUG> [<WORKSHOP>/<SDK>][:<SLOT>] [OPTIONS]


Synopsis
--------

This command connects a plug to a target slot
that is specified as the second argument or deduced from the context.

- If the second argument is omitted entirely, the target is assumed to be
  :samp:`<WORKSHOP>/agent:<PLUG>`;
  :samp:`<WORKSHOP>` and :samp:`<PLUG>` come from the first argument

- If the second argument only names the slot itself, the target is
  :samp:`<WORKSHOP>/agent:<SLOT>`;
  :samp:`<WORKSHOP>` comes from the first argument

- If the second argument only names the workshop and SDK, the target is
  :samp:`<WORKSHOP>/<SDK>:<INTERFACE>`;
  :samp:`<INTERFACE>` comes from the plug definition.
  However, if there are several candidate slots that use this interface,
  the command fails

- If the target slot is compatible with the plug, the command attempts
  to connect them and returns the result


Notes
-----

- To be compatible, the plug and the slot must use the same interface
- Multiple plugs can be connected to the same slot, but not vice versa
- The :ref:`ref_workshop_connections` output
  will list the connection as :samp:`manual`


Options
-------

--no-wait

  Return the change ID, don't wait for the operation to finish.


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
- :ref:`exp_interfaces_plugs_slots`
- :ref:`exp_sdk`


Reference:

- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_info`
