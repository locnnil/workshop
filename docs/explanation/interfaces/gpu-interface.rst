.. _exp_gpu_interface:

GPU interface
=============

The GPU interface
enables GPU pass-through
(direct access to the host system's GPUs)
inside the workshop
to improve the performance of GPU-intensive applications.

By using the GPU interface,
the SDK publisher allows the workshop to directly access the host's GPU devices,
which may be required for various GPU-intensive workloads.


Connection
----------

The interface is connected automatically at launch and refresh;
also,
the :command:`workshop connect` and :command:`workshop disconnect` commands
can be invoked manually.

Establishing a connection means
the host's GPUs are directly available inside the workshop
via the GPU pass-through mechanism.

To check if the interface is connected:

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

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_shell`
