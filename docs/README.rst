Workspace
=========

**Workspace is a tool that automates intricate prerequisite setup
for your projects**.

**Define your dev environment in straightforward YAML**.
The tool consumes the definition to create a contained workspace,
installs the dependencies it lists as a number of SDKs,
and attaches their life cycle hooks for run-time control.
IDEs such as Visual Studio Code or Jupyter Lab
can discover workspaces and leverage them in their operation,
tidying up your system and streamlining your work.

**Untangle the know-how that was weaved into your project**.
An environment that could take hours of setup
can be launched with one command;
workspaces enhance issue reproduction across platforms,
facilitate collaboration in code reviews,
and confine hackish experiments in lightweight containers.

**Mitigate your setup's complexity with Workspace.**
AI/ML, robotics, IoT, EdTech, and similar domains
commonly have less-than-trivial project layouts
that depend on multiple Linux distributions,
a plethora of SDKs from different publishers,
and a grocery list of libraries and programming languages.


Getting Started
---------------

Follow the sections below
or refer to the
`Getting started
<https://canonical-workspace.readthedocs-hosted.com/en/latest/tutorial/getting-started/>`_
tutorial series for a more detailed introduction to Workspace.


------------
Installation
------------

Workspace requires
`LXD <https://documentation.ubuntu.com/lxd/en/latest/>`_
for low-level operation:

.. code-block:: bash

   sudo snap install lxd
   sudo lxd init --auto


Next, install Workspace from source code:

.. code-block:: bash

   go install github.com/canonical/workspace/cmd/workspaced@latest
   go install github.com/canonical/workspace/cmd/workspace@latest


Running Workspace
-----------------

To run the daemon,
create a directory to store Workspace state and data,
save its pathname in the ``$WORKSPACE`` environment variable,
and type ``workspaced run``:

.. code-block:: bash

   mkdir ~/workspace
   export WORKSPACE=~/workspace
   workspaced run

     2023-08-17T01:37:23.962Z [workspaced] Started daemon.
     ...


Launching workspaces
--------------------

In the root directory of the project
that you want to use with Workspace,
create a workspace definition file named ``.workspace.<NAME>.yaml``
to list your project's prerequisites
and run ``workspace launch <NAME>``:

.. code-block:: bash

   cat > .workspace.nimble.yaml <<EOF -
   name: nimble
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
     openjdk:
       channel: latest/stable
   EOF

   workspace launch nimble


Workspace downloads and installs the SDKs your definition lists;
the project is now ready to use them.


Testing
-------

Workspace uses a ``go test``-compatible
`gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_:

.. code-block:: bash

   go test ./...
   go test -check.f <TestName|SuiteName>


To run end-to-end tests and integration tests with
`Spread <https://github.com/snapcore/spread>`_:

.. code-block:: bash

   go install github.com/snapcore/spread/cmd/spread@latest
   spread
