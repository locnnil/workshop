.. _ref_workshop_sketch-sdk:

workshop sketch-sdk
-------------------

Edit the sketch SDK and graft it onto the workshop.

.. rubric:: Synopsis

.. code-block:: console

   $ workshop sketch-sdk [--stash|--restore|--remove] <WORKSHOP> [flags]

.. rubric:: Description


This command opens the default text editor to configure the 'sketch' SDK
and immediately installs it in the specified workshop,
enabling rapid experiments and tweaks at the SDK level.

Saving and exiting causes a refresh,
which installs the updated 'sketch' SDK in the workshop.

The '--stash' and '--restore' options stash the 'sketch' SDK,
reversing the changes, and quickly restore it to the workshop.

The '--remove' option removes the 'sketch' SDK permanently.

Notes:

- The 'sketch' SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts

- In addition to hooks, the 'sketch' SDK can use interfaces,
  define plugs, slots, connections and bindings

- You can partially refresh the workshop, targeting the 'sketch' SDK
  with the 'workshop refresh <WORKSHOP>/sketch' command


.. rubric:: Options


--remove

   Remove the sketch SDK from the workshop.


--restore

   Return the previously stashed SDK to the workshop.


--stash

   Stash the sketch SDK and remove it from the workshop.
