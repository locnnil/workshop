Project, workspace, SDKs
========================

Projects, workspaces, workspace definitions and SDKs
are the key building blocks of |project|.


Project
-------

A project is a directory that contains one or many workspace definitions.

When a workspace runs,
this directory is automounted as ``/project/``;
the changes to the directory are tracked
to keep the workspace configuration in sync.
Thus, if the directory is moved or copied,
the mount points in related workspaces are updated.

If the directory is deleted,
the workspaces that still refer to it
switch to the *Error* state
and become unavailable for any commands except ``remove``.


.. _exp_workspace:

Workspace
---------

A *workspace* is a container that is described in a definition file.


Workspace definition
~~~~~~~~~~~~~~~~~~~~

This is a file named ``.workspace.<NAME>.yaml``
that lists the base image of the workspace
and the specific components installed on top of it.
The definition serves as a single source of truth about the workspace.
It usually takes a few tries
to arrive at a configuration that suits your project,
so you can edit and update the workspace definition iteratively.

A simple definition may look like this:

.. code-block:: yaml

   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable

This specifies a *base* and an *SDK*.
A more complete defintion would usually list
multiple SDKs, interfaces, packages and life cycle hooks.


Base image
~~~~~~~~~~

The base is a supported OS image
that is used as the foundation of the workspace.
Currently, it can be either ``ubuntu@20.04`` or ``ubuntu@22.04``.



SDKs
----

SDKs are essential workspace components
that install the required system and language packages,
configure the workspace for their operation
and maintain their own state
throughout the lifetime of the workspace.
An *SDK* is designed by a publisher
and made available via the SDK Store.
A single workspace can include multiple SDKs from different publishers.
SDKs are distributed via channels similar to
`snap channels <https://snapcraft.io/docs/channels>`_.

An SDK has a state that persists SDK-specific data,
such as a model training configuration.
|project| saves the state before applying any changes to the SDK,
such as in a :ref:`refresh <tut_refresh>` operation.
After a successful change, the state is restored.
The specific save and restore actions
are implemented by the publisher.
