.. _exp_interface_concepts:

Interface concepts
==================

.. @artefact SDK
.. @artefact interface

In |ws_markup|, SDKs can act as providers and consumers of resources
such as the GPU or file directories.
Host system resources
are exposed via the :ref:`system SDK <exp_system_sdk>`
that is present in every workshop by design.

For a workshop to be operational, the SDKs listed in its definition
must *connect* to the resources they expect.
Such connections are uniformly established via the interface system.


.. _exp_interface_connections:

Connections
-----------

.. @artefact interface connection

Interface connections are a mechanism for communication and resource sharing.
It is an integral part of workshop confinement,
ensuring that each workshop operates in its own isolated environment,
while still allowing controlled interactions among the SDKs and with the system.

Here's how it works from the outside:

- The :samp:`connections` section of the workshop definition
  and the :command:`workshop connect` command
  can be used to link interface plugs to respective slots,
  allowing the SDKs to orderly access the resources.

- Conversely, the :command:`workshop disconnect` command
  terminates existing interface connections,
  revoking the access to the resources granted by the connection.

- Finally, the :command:`workshop connections` command
  lists all existing connections and their states,
  providing an overview of how workshop connections are laid out.

Some plugs can be auto-connected to their slots at launch or refresh.
This behaviour varies by interface,
but the overall aim is to conduct reasonably in each case:
the :ref:`mount <exp_mount_interface>`
and the :ref:`GPU <exp_gpu_interface>` interfaces are auto-connected,
whereas the :ref:`camera <exp_camera_interface>`,
:ref:`desktop <exp_desktop_interface>` and :ref:`SSH <exp_ssh_interface>`
interfaces require manual connection.


.. _exp_plug_bindings:

Plug bindings
-------------

When you list an SDK in your workshop,
you can optionally *bind* any of its plugs
to other plugs of the same :ref:`interface <exp_interfaces>`
in the same workshop.

Binding a plug to another plug makes them both refer to a single entity;
any action on a bound plug affects all bindings, and vice versa.
This comes handy if the SDKs implement different features on the same resources
or simply use a singleton-like interface (:samp:`gpu` is a good example).

.. @artefact SDK publisher

.. note::

   Double-check the plug layout
   with :command:`workshop connections`;
   you may also need additional info from the SDK publishers.


As an example,
imagine two SDKs, :samp:`pytorch` and :samp:`tensorflow`,
that store their training data in the workshop under
:file:`~/.cache/torchvision/datasets/` and :file:`~/.keras/datasets/`,
respectively.
The data should be persisted,
so each SDK has a corresponding mount interface plug, :samp:`datasets`.

Now, what if our workshop includes both SDKs;
can we leverage bindings to reuse the data?
Here, the :samp:`datasets` plug of the :samp:`pytorch` SDK
is bound to the :samp:`datasets` plug under :samp:`tensorflow`:

.. code-block:: yaml
   :caption: .workshop/digits.yaml
   :emphasize-lines: 8

   name: digits
   base: ubuntu@22.04
   sdks:
     - name: pytorch
       channel: latest/stable
       plugs:
         datasets:
           bind: tensorflow:datasets
     - name: tensorflow
       channel: latest/stable


This binds :samp:`pytorch:datasets`
to the location of :samp:`tensorflow:datasets`;
you benefit from sharing the data between the two frameworks,
while simultaneously persisting it on the host.

Any actions on the plug thus bound affect all its bindings.
Here, if you remount :samp:`pytorch:datasets`,
the :samp:`tensorflow:datasets` plug is also remounted
because they reference the same entity:

.. @artefact workshop info

.. code-block:: console

   $ workshop remount digits/pytorch:datasets /new-mount/
   $ workshop info digits

     ...
     sdks:
       pytorch:
         tracking:   latest/stable
         installed:  2.5.1  2024-11-02  (42)
         mounts:
           datasets:
             host-source:      /new-mount
             workshop-target:  /home/workshop/.cache/torchvision/datasets
       tensorflow:
         tracking:   latest/stable
         installed:  2.18.0  2024-10-27  (37)
         mounts:
           data:
             host-source:      /new-mount
             workshop-target:  /home/workshop/.keras/datasets


This avoids the need to reconfigure each mount manually,
reducing the potential for mistakes.

When you run :command:`workshop connections`,
a bound plug will have :samp:`bind` listed under :samp:`Notes`,
along with the line number of the target plug:

.. @artefact workshop connections

.. code-block:: console

   $ workshop connections digits

     Interface  Plug                        Slot                 Notes
     mount      digits/pytorch:datasets     digits/system:mount  bind.2
     mount      digits/tensorflow:datasets  digits/system:mount  bind.2


Here, both plugs are listed as :samp:`bind.2`,
pointing to :samp:`tensorflow:datasets` in the second line.


See also
--------

Explanation:

- :ref:`exp_workshop`
- :ref:`exp_sdk`


Reference:

- :ref:`ref_cli`
- :ref:`ref_workshop_connect`
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_definition`
- :ref:`ref_workshop_disconnect`