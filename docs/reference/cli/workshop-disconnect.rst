.. _ref_workshop_disconnect:

workshop disconnect
===================

Disconnects a plug or a slot.

.. code-block:: console

   $ workshop disconnect <WORKSHOP>/<SDK>:<PLUG OR SLOT> [<WORKSHOP>/<SDK>]:[<SLOT>] [OPTIONS]


Synopsis
--------

This command disconnects a plug from its slot, or a slot from all its plugs.

- A single argument can be a fully qualified plug or slot reference;
  with two arguments, the first one is the plug, and the second one is the slot

- If the second argument only names the slot itself, the target is
  :samp:`<WORKSHOP>/agent:<SLOT>`;
  :samp:`<WORKSHOP>` comes from the first argument

- If the second argument only names the workshop and SDK, the target is
  :samp:`<WORKSHOP>/<SDK>:<INTERFACE>`;
  :samp:`<INTERFACE>` comes from the plug definition


Notes
-----

- After an auto-connected plug is thus disconnected,
  it is reconnected during :ref:`ref_workshop_refresh`
  only if the :option:`!--forget` option was used
  with :command:`workshop disconnect`


Options
-------

--forget

  Reconnect the plugs at :ref:`ref_workshop_refresh`
  if auto-connected initially.

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
- :ref:`exp_plugs_slots`
- :ref:`exp_sdk`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_refresh`
