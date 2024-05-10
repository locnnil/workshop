.. _exp_gpu_interface:

GPU interface
=============

The GPU interface
enables GPU passthrough
(direct access to the host system's GPUs)
inside the workshop
to improve the performance of GPU-intensive applications.


GPU interface plug
------------------

An essential element here is the GPU interface plug
that is declared in the SDK definition.

.. important::

   An SDK definition, usually named :file:`sdkcraft.yaml`,
   is different from a
   :ref:`workshop definition <exp_workshop_def>`,
   usually named :file:`.workshop.<NAME>.yaml`;
   the former is used to build SDKs with `SDKcraft`_
   and isn't normally needed with |project_markup|,
   whereas the latter is a crucial element of daily |project_markup| activities.

   The following example is provided only to detail how the GPU interface works.


A basic structure includes the name of the plug itself
and the name of the interface (:samp:`gpu` in this case):

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: gpu-sdk
   title: GPU SDK
   base: ubuntu@22.04
   summary: The GPU SDK
   description: |
     GPU SDK serves to demonstrate how the GPU interface works.

   plugs:
     gpu-plug:
       interface: gpu


This definition creates a plug called :samp:`gpu-plug`
that sets its :samp:`interface` type to :samp:`gpu`,
which makes it (surprise!) a GPU interface plug.


GPU interface slot
------------------

To let SDKs in a workshop access the host system's GPUs directly,
|project_markup| creates a GPU interface slot,
which multiple GPU interface plugs can then access.

When the SDK is installed
during :command:`workshop launch` and :command:`workshop refresh`,
|project_markup| checks the following for each plug that targets the slot:

- The plug can be installed.

- The plug can be auto-connected
  (for :samp:`gpu`, it's a yes).


If the plug passes the checks, it's successfully connected to the slot:

.. code-block:: console

   $ workshop connections --all

       Interface  Plug                   Slot      Notes
       ...
       gpu        ws/gpu-sdk:gpu         :gpu      -


See also
--------

Explanation:

- :ref:`exp_sdk_definition`
- :ref:`exp_interfaces_plugs_slots`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
