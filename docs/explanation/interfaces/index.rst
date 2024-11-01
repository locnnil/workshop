Interfaces
==========

These articles explain concepts
that are important for understanding |project_markup|'s interface mechanics.

.. toctree::
   :glob:
   :maxdepth: 1

   *


.. _exp_interface_connections:

Summary
-------

In |project_markup|, SDKs can act as providers and consumers of resources
such as the GPU or file directories.
Host system resources
are exposed via the :ref:`system SDK <exp_system_sdk>`
that is present in every workshop by design.

For a workshop to be operational, the SDKs listed in its definition
must *connect* to the resources they expect.
Such connections are uniformly established via the interface system.

Interface connections are a mechanism for communication and resource sharing.
It is an integral part of workshop confinement,
ensuring that each workshop operates in its own isolated environment,
while still allowing controlled interactions among the SDKs and with the system.

Here's how it works from the outside:

- The :samp:`connections` section of the workshop definition
  and the :command:`workshop connect` command
  can be used to link interface plugs to respective slots,
  allowing the SDKs to orderly access the resources.

- Conversely, the :command:`workshop disconnect` command
  terminates existing interface connections,
  revoking the access to the resources granted by the connection.

- Finally, the :command:`workshop connections` command
  lists all existing connections and their states,
  providing an overview of how workshop connections are laid out.

Some plugs can be auto-connected to their slots at launch or refresh.
This behaviour varies by interface,
but the overall aim is to conduct reasonably in each case:
the :ref:`mount <exp_mount_interface>`
and the :ref:`GPU <exp_gpu_interface>` interfaces are auto-connected,
whereas the :ref:`camera <exp_camera_interface>`
and :ref:`SSH <exp_ssh_interface>` interfaces require manual connection.


.. _exp_system_sdk:

System SDK
~~~~~~~~~~

Every workshop contains a special *system SDK*
that exposes system resources through its slots.
It's unavailable from the SDK Store;
installed first at :command:`workshop launch`
and removed last during :command:`workshop remove`,
it ensures internal consistency.

The purpose of the system SDK isn't to add hooks or additional content;
it's only there to uniformly expose host system resources to other SDKs.
As such, it can't be removed by the user
and isn't listed in the :command:`workshop info` output.
It's also the only SDK
that can have :ref:`mount interface <exp_mount_interface>` slots on the host.


See also
--------

Explanation:

- :ref:`exp_sdk_definition`
- :ref:`exp_plugs_slots`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_disconnect`
- :ref:`ref_workshop_info`
- :ref:`ref_workshop_launch`
- :ref:`ref_workshop_refresh`
- :ref:`ref_workshop_remove`
