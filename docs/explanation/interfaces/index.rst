.. meta::
   :description: Workshop interfaces documentation, providing
                 access to explanations of interface concepts and specific
                 interface types used for resource sharing between containers.

Interfaces
==========

Interfaces allow communication and resource sharing
between a workshop and the host system,
as well as between the different SDKs that are part of a workshop.
These articles explain the mechanics behind interfaces
and describe each supported interface type.


General concepts
----------------

Start here to understand how interfaces use plugs and slots
to connect SDKs to host resources and to each other:

.. toctree::
   :maxdepth: 1

   concepts


Hardware interfaces
-------------------

These interfaces provide workshops with access
to host hardware such as displays, GPUs, and cameras:

.. toctree::
   :maxdepth: 1

   camera-interface
   desktop-interface
   gpu-interface


Data and connectivity
---------------------

These interfaces handle filesystem access, SSH agent forwarding,
and network service sharing between workshops and the host:

.. toctree::
   :maxdepth: 1

   mount-interface
   ssh-interface
   tunnel-interface
