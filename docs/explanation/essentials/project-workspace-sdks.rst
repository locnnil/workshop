Project, workspace, SDKs
========================

The concepts of a project, its workspaces, their definitions and SDKs are the
most important aspects of |project|.


Project
-------

The project directory will be mounted into the container automatically under
the ``/project`` pathname. Workspace tracks changes to the project directory
keep the container configuration in sync. Thus, if the project directory is
moved, copied or deleted, the corresponding workspace container will update
its mounts automatically. If the directory is removed, the remaining
workspaces that still exist will be switched to the *Error* state and become
unavailable for any commands except ``remove``. Try moving the project
directory and check the results of the ``workspace list`` output.


.. _exp_workspace:

Workspace
---------

A *workspace* is a system container that is created, configured and launched
from a definition provided in a workspace file. The file is a straightforward
YAML specification that describes the container's base and SDKs that will be
installed into the workspace. It is intended to be created and maintained as a
single source of truth about the development environment of your project.

The workspace file of an actual project may contain multiple SDKs, interfaces,
packages and life cycle hooks. When approached for the first time, it is likely
that designing a workspace will take a few iterations before arriving at the
desired development environment for your project.

.. note::

   It is a good idea to keep your locally running workspace instance in sync
   with that of your team, by using the project's workspace file as a single
   source of truth for your development environment.

A workspace file must be named as follows: ``.workspace.<NAME>.yaml``.
The contents of a simple file may look like this:

.. code-block:: yaml

    name: nimble
    base: ubuntu@22.04
    sdks:
        go:
            channel: latest/stable

This adds two related concepts: *base* and *SDK*.
Here, *base* can be either ``ubuntu@20.04`` or ``ubuntu@22.04``. It is a
supported OS that will be used to create the workspace container.


SDK
---

An *SDK* is a Software Development Kit designed by a publisher and available in
the SDK Store. The SDK is a building block for your workspace that installs the
required system and language packages, configures the environment and maintains
its state throughout the lifetime of the workspace. A workspace can contain
multiple SDKs from various publishers. In this example, we use a simple Go
language SDK.

SDKs are distributed via channels, the concept that reproduces the semantics of
a `snap channel <https://snapcraft.io/docs/channels>`_.

Any SDK has a notion of state that will be preserved over its life cycle. If an
SDK has state data, for example a specific training configuration, Workspace
saves the state before any refresh operation starts. The state is restored in
the refreshed workspace. Both save and restore scripts are provided by the SDK
author.
