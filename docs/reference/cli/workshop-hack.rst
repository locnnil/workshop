.. _ref_workshop_sketch_sdk:

<<<<<<< HEAD
workshop hack
-------------

Edit the hack SDK and graft it onto the workshop.

.. rubric:: Synopsis

.. code-block:: console

   $ workshop hack [--drop|--restore] <WORKSHOP> [setup-base|save-save|restore-state|check-health] [flags]

.. rubric:: Description
=======
workshop sketch-sdk
=============

Edits the sketch SDK and grafts it onto the workshop.

.. code-block:: console

   $ workshop sketch-sdk <WORKSHOP> [OPTIONS]
>>>>>>> 1b9a21d (Rename workshop hack to workshop sketch-sdk)


This command opens the default text editor to configure the 'hack' SDK
and immediately installs it in the specified workshop,
enabling rapid experiments and tweaks at the SDK level.

<<<<<<< HEAD
If <HOOK> isn't specified, the command opens the SDK definition file.
Setting the <HOOK> value opens the respective hook file:

- 'check-health'
- 'restore-state'
- 'save-state'
- 'setup-base'


Saving and exiting causes a refresh,
which installs the updated 'hack' SDK in the workshop.

The '--drop' and '--restore' options stash the 'hack' SDK,
reversing the changes, and quickly restore it to the workshop.


Notes:

- The 'hack' SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts

- In addition to hooks, the 'hack' SDK can use interfaces,
  define plugs, slots, connections and bindings

- You can partially refresh the workshop, targeting the 'hack' SDK
  with the 'workshop refresh <WORKSHOP>/hack' command


.. rubric:: Options


--drop

   Drop the hack SDK from the workshop.


--restore

   Return the previously dropped SDK to the workshop.



.. rubric:: Examples


Edit the hack SDK definition for the 'nimble' workshop
=======
Edit the :samp:`sketch` SDK definition for the :samp:`nimble` workshop
>>>>>>> 1b9a21d (Rename workshop hack to workshop sketch-sdk)
and apply it after saving by automatically refreshing the workshop:

.. code-block:: console

   $ workshop sketch-sdk nimble

<<<<<<< HEAD

Edit the 'check-health' hook for the hack SDK
and apply it after saving by automatically refreshing the workshop:

.. code-block:: console

   $ workshop hack nimble check-health


Stash the hack SDK, temporarily reverting the changes in the workshop:

.. code-block:: console

   $ workshop hack nimble --drop


=======
Stash the :samp:`sketch` SDK, temporarily reverting the changes in the workshop:

.. code-block:: console

   $ workshop sketch-sdk nimble --drop


Synopsis
--------

This command opens the default text editor to configure the :samp:`sketch` SDK
and immediately installs it in the specified workshop,
enabling rapid experiments and tweaks at the SDK level.

Saving and exiting causes a refresh,
which installs the updated :samp:`sketch` SDK in the workshop.

The :option:`!--drop` and :option:`!--restore` options
stash the :samp:`sketch` SDK, reversing the changes,
and quickly restore it to the workshop.


Notes
-----

- The :samp:`sketch` SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts

- In addition to hooks, the :samp:`sketch` SDK can use interfaces,
  define plugs, slots, connections and bindings

- You can partially refresh the workshop, targeting the :samp:`sketch` SDK
  with the :command:`workshop refresh <WORKSHOP>/sketch` command


Options
-------

--drop

  Drop the :samp:`sketch` SDK from the workshop.

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
>>>>>>> 1b9a21d (Rename workshop hack to workshop sketch-sdk)
