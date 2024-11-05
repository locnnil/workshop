.. _ref_workshop_hack:

workshop hack
=============

Edits the hack SDK and grafts it onto the workshop.

.. code-block:: console

   $ workshop hack <WORKSHOP> [<HOOK>] [OPTIONS]


Examples
--------

Edit the :samp:`hack` SDK definition for the :samp:`nimble` workshop
and apply it after saving by automatically refreshing the workshop:

.. code-block:: console

   $ workshop hack nimble


Edit the :samp:`check-health` hook for the :samp:`hack` SDK
and apply it after saving by automatically refreshing the workshop:

.. code-block:: console

   $ workshop hack nimble check-health


Stash the :samp:`hack` SDK, temporarily reverting the changes in the workshop:

.. code-block:: console

   $ workshop hack nimble --drop


Synopsis
--------

This command opens the default text editor to configure the :samp:`hack` SDK
and immediately installs it in the specified workshop,
enabling rapid experiments and tweaks at the SDK level.

If :samp:`<HOOK>` isn't specified, the command opens the SDK definition file.
Setting the :samp:`<HOOK>` value opens the respective hook file:

- :samp:`check-health`
- :samp:`restore-state`
- :samp:`save-state`
- :samp:`setup-base`


Saving and exiting causes a refresh,
which installs the updated :samp:`hack` SDK in the workshop.

The :option:`!--drop` and :option:`!--restore` options
stash the :samp:`hack` SDK, reversing the changes,
and quickly restore it to the workshop.


Notes
-----

- The :samp:`hack` SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts

- In addition to hooks, the :samp:`hack` SDK can use interfaces,
  define plugs, slots, connections and bindings

- You can partially refresh the workshop, targeting the :samp:`hack` SDK
  with the :command:`workshop refresh <WORKSHOP>/hack` command


Options
-------

--drop

  Drop the :samp:`hack` SDK from the workshop.

--restore

  Restore the previously dropped SDK to the workshop.


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
- :ref:`exp_interface_connections`
- :ref:`exp_plug_bindings`
- :ref:`exp_sdk`
- :ref:`exp_workshop_def`

Reference:

- :ref:`ref_workshop_refresh`
