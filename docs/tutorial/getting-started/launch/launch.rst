Creating your first workspace
=============================

Workspace is a system container that is created, configured and launched from a
definition provided in a workspace file. The file is a straightforward YAML
specification that describes the container's base and SDKs that will be
installed into the workspace. It is intended to be created and maintained as a
single source of truth about the development environment of your project. Thus,
on-boarding or reproducing the environment on a new machine would be as simple
as ``workspace launch``.

We'll start with exploring the concepts of workspace, SDK and workspace file.
These are the most important aspects of the tool.

Let's create a project directory ``hello-workspace`` with a simple workspace
file ``.workspace.nimble.yaml`` in it:

.. code-block:: yaml

    name: nimble
    base: ubuntu@22.04
    sdks:
        go:
            channel: latest/stable

.. note::
    A Workspace file must satisfy the following naming convention: .workspace.\ *name*\ .yaml

Our ``nimble`` workspace introduces two concepts: *base* and *SDK*.

*base* can be either ``ubuntu@20.04`` or ``ubuntu@22.04``. It is a supported OS
that will be used to create the workspace container.

*SDK* is a Software Development Kit designed by a publisher and available in the
SDK Store. The SDK is a building block for your workspace that installs the
required system and language packages, configures the environment and maintains
its state throughout the lifetime of the workspace. A workspace can contain
multiple SDKs from various publishers. In this example, we use a simple Go
language SDK.

The SDKs are distributed via channels, the concept that reproduces the semantics
of a `snap channel
<https://snapcraft.io/docs/channels#:~:text=Channels%20are%20an%20important%20snap,under%20the%20same%20snap%20name>`_.

Launch
------

Now Workspace should be able to find the newly created workspace in an *Off*
state. To confirm, run the following command from the project directory:

.. code-block:: bash

    $ workspace list
    Project                 Workspace  State  Notes
    ~/Work/hello-workspace  nimble     Off    -

We are ready to launch the ``nimble`` workspace:

.. code-block:: bash

    $ workspace launch nimble
    "nimble" launched

Done! Once you have launched a workspace, it can be used to build, debug and run
code either from your favourite IDE or directly from the command line.

.. note::

    The project directory will be mounted into the container automatically under
    the ``/project`` pathname. Workspace tracks the project directory changes to
    keep the container configuration in sync. Thus, if the project directory is
    moved, copied or deleted, the corresponding workspace container will update
    its mounts automatically. If the directory is removed, the remaining
    workspaces that still exist will be switched to the *Error* state and become
    unavailable for any commands except ``remove``. Try move around the project
    directory and check the results of the ``workspace list`` output.


Starting and stopping
---------------------

A workspace will be in the *Ready* state if the launch was successful. When
not used, stop the workspace by running:

.. code-block:: bash

    $ workspace stop nimble
    "nimble" stopped

Both ``workspace start`` and ``workspace stop`` commands wait for the graceful
completion of the operation  and cannot be interrupted from the command-line in
order to ensure the workspace integrity.
