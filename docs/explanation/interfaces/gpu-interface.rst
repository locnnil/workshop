.. _exp_gpu_interface:

GPU interface
=============

The GPU interface
enables GPU pass-through
(direct access to the host system's GPUs)
inside the workshop
to improve the performance of GPU-intensive applications.


GPU interface plug
------------------

An essential element here is the content interface plug,
which is declared in the :ref:`SDK definition <exp_sdk_definition>`
and is thus beyond the reach of |project_markup|.
By adding it, the SDK publisher enables the workshop
to directly access the host's GPU devices,
which may be required for various GPU-intensive workloads.


GPU interface slot
------------------

To enable this mechanism,
|project_markup| provides a GPU interface slot
to which multiple GPU interface plugs can
:ref:`connect <exp_interface_connections>`.

When an SDK is installed
during :command:`launch` and :command:`refresh`,
|project_markup| checks that the plug targeting the slot
passes :ref:`validation <exp_interfaces_validation>`.
If the plug passes these checks,
it is automatically connected.

To ensure the plug has connected to the slot:

.. code-block:: console

   $ workshop connections --all

     Interface  Plug                   Slot      Notes
     ...
     gpu        ws/gpu-sdk:gpu         :gpu      -


This means the host's GPUs are directly available inside the workshop:

.. code-block:: console

   $ workshop shell ws
   workshop@ws-8584e571$ ls -h /dev/dri/

     card0  renderD128

   workshop@ws-8584e571$ nvidia-smi


See also
--------

Explanation:

- :ref:`exp_interfaces`
- :ref:`exp_plugs_slots`
- :ref:`exp_sdk_definition`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_shell`
