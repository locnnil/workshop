.. _exp_workshop:

Workshops
=========

A *workshop*
(lowercase; not to be confused with |project_markup| itself)
is a container that is described in a definition file,
which is associated with a :ref:`project directory <exp_projects>`.
Currently, these containers are hosted by
`LXD <https://documentation.ubuntu.com/lxd/en/latest/>`__,
but it's not recommended to rely on this implementation detail.


.. _exp_workshop_def:

Workshop definition
-------------------

This is a file named :file:`.workshop.<NAME>.yaml`
that lists the base image of the workshop
and the specific components installed on top of it.
The definition acts as a single source of truth about the workshop.
It usually takes a few tries
to produce a definition that works for your project,
so you can edit and update the file iteratively.

A simple definition might look like this:

.. code-block:: yaml
   :caption: .workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable


It specifies a *base* and an *SDK*.
A more complete definition would usually list several SDKs
that use different :ref:`interfaces <exp_interfaces>`,
software packages and :ref:`hooks <exp_sdk_hooks>`.


.. _exp_base:

Base image
----------

The base is a supported OS image
that is used as the basis for the workshop.
Currently, it can be either
:samp:`ubuntu@20.04`, :samp:`ubuntu@22.04` or :samp:`ubuntu@24.04`.


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
so each SDK has a corresponding content interface plug, :samp:`datasets`.

Now, what if our workshop includes both SDKs;
can we leverage bindings to reuse the data?
Here, the :samp:`datasets` plug of the :samp:`pytorch` SDK
is bound to the :samp:`datasets` plug under :samp:`tensorflow`:

.. code-block:: yaml
   :caption: .workshop.digits.yaml
   :emphasize-lines: 8

   name: digits
   base: ubuntu@22.04
   sdks:
     pytorch:
       channel: latest/stable
       plugs:
         datasets:
           bind: tensorflow:datasets
     tensorflow:
       channel: latest/stable


This binds :samp:`pytorch:datasets`
to the location of :samp:`tensorflow:datasets`;
you benefit from sharing the data between the two frameworks,
while simultaneously persisting it on the host.

Any actions on the plug thus bound affect all its bindings.
Here, if you remount :samp:`pytorch:datasets`,
the :samp:`tensorflow:datasets` plug is also remounted
because they reference the same entity:

.. code-block:: console

   $ workshop remount digits/pytorch:datasets /new-mount/
   $ workshop info digits

     ...
     sdks:
       pytorch:
         channel: latest/stable
         mounts:
           datasets:
             host:      /new-mount
             workshop:  /home/workshop/.cache/torchvision/datasets/
       tensorflow:
         channel: latest/stable
         mounts:
           data:
             host:      /new-mount
             workshop:  /home/workshop/.keras/datasets/


This avoids the need to reconfigure each mount manually,
reducing the potential for mistakes.

When you run :command:`workshop connections`,
a bound plug will have :samp:`bind` listed under :samp:`Notes`,
along with the line number of the target plug:

.. code-block:: console

   $ workshop connections digits

     Interface  Plug                        Slot      Notes
     content    digits/pytorch:datasets     :content  bind.2
     content    digits/tensorflow:datasets  :content  bind.2


Here, both plugs are listed as :samp:`bind.2`,
pointing to :samp:`tensorflow:datasets` in the second line.


See also
--------

Explanation:

- :ref:`exp_projects`
- :ref:`exp_sdk`


Reference:
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_def_yaml`
