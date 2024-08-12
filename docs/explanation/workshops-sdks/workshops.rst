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

   To use binding, you have to know the plug layout of the SDKs
   and may need additional instructions from the SDK publisher.


In the following example,
the workshop uses two SDKs, :samp:`go` and :samp:`dev-tunnel`;
the :samp:`data` plug, presumably defined by the :samp:`dev-tunnel` SDK,
is bound to the :samp:`mod-cache` plug, assumed to exist in the :samp:`go` SDK:

.. code-block:: yaml
   :caption: .workshop.go-dev.yaml

   name: go-dev
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/candidate
     dev-tunnel:
       channel: latest/edge
       plugs:
         data:
           bind: go:mod-cache


This binds :samp:`dev-tunnel:data` to the location of :samp:`go:mod-cache`,
so the :samp:`dev-tunnel` SDK can now apply its logic to the data there.
For instance, if :samp:`dev-tunnel` synchronises its data with the cloud,
it will independently occur on top of caching provided by :samp:`go`.

Any actions on the plug thus bound affect all its bindings.
In our example, if you remount :samp:`dev-tunnel/data`,
the :samp:`go:mod-cache` plug is remounted as well
because they're just two aliases of the same entity:

.. code-block:: console

   $ workshop remount go-dev/dev-tunnel:data /new-mount/
   $ workshop info go-dev

     ...
     sdks:
       go:
         channel: latest/candidate
         mounts:
           mod-cache:
             host:      /new-mount/
             workshop:  /data
       dev-tunnel:
         channel: latest/edge
         mounts:
           data:
             host:      /new-mount/
             workshop:  /data


This avoids the need to reconfigure each mount manually,
reducing the potential for mistakes.

When you run :command:`workshop connections`,
a bound plug will have :samp:`bind` listed under :samp:`Notes`,
along with the line number of the target plug:

.. code-block:: console

   $ workshop connections go-dev

     Interface  Plug                     Slot      Notes
     content    go-dev/dev-tunnel:data   :content  bind.2
     content    go-dev/go:mod-cache      :content  bind.2


Here, both plugs are listed as :samp:`bind.2`, so they point to the second line
because they are just aliases of the same entity, :samp:`go:mod-cache`.


See also
--------

Explanation:

- :ref:`exp_projects`
- :ref:`exp_sdk`


Reference:
- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_def_yaml`
