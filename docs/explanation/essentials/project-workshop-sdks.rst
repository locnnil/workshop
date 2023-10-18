Project, workshop, SDKs
========================

Projects, workshops, workshop definitions and SDKs
are the key building blocks of |project|.


.. _exp_project:

Project
-------

A project is a directory that contains one or many workshop definitions.

When a workshop runs,
this directory is mounted as :file:`/project/`;
the changes to the directory are tracked
to keep the workshop configuration in sync.
Thus, if the directory is moved or copied,
the mount points in related workshops are updated.

If the directory is deleted,
the workshops that still refer to it
switch to the *Error* state
and become unavailable for any commands except :command:`remove`.


.. _exp_workshop:

Workshop
---------

A *workshop* is a container that is described in a definition file.


.. _exp_workshop_def:

Workshop definition
~~~~~~~~~~~~~~~~~~~~

This is a file named :file:`.workshop.<NAME>.yaml`
that lists the base image of the workshop
and the specific components installed on top of it.
The definition serves as a single source of truth about the workshop.
It usually takes a few tries
to arrive at a configuration that suits your project,
so you can edit and update the workshop definition iteratively.

A simple definition may look like this:

.. code-block:: yaml

   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable

This specifies a *base* and an *SDK*.
A more complete definition would usually list
multiple SDKs, interfaces, packages and life cycle hooks.


.. _exp_workshop_base:

Base image
~~~~~~~~~~

The base is a supported OS image
that is used as the foundation of the workshop.
Currently, it can be either ``ubuntu@20.04`` or ``ubuntu@22.04``.


.. _exp_sdk:

SDKs
----

SDKs are essential workshop components
that install the required system and language packages,
configure the workshop for their operation
and maintain their own state
throughout the lifetime of the workshop.
An *SDK* is designed by a publisher
and made available via the SDK Store.
A single workshop can include multiple SDKs from different publishers.
SDKs are distributed via channels similar to
`snap channels <https://snapcraft.io/docs/channels>`_.

An SDK has a state that persists SDK-specific data,
such as a model training configuration.
|project| saves the state before applying any changes to the SDK,
such as in a :ref:`refresh <tut_refresh>` operation.
After a successful change, the state is restored.
The specific save and restore actions
are implemented by the publisher.
