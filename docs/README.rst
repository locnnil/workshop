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

Start with the `Getting Started <https://canonical-workspace.readthedocs-hosted.com/en/latest/tutorial/>`_ for the full introduction into the
workspace installation and essentials.

---------

Install
-------

Workspace relies on LXD to orchestrate containers:

.. code-block:: bash

  sudo snap install lxd
  sudo lxd init --auto


Install Workspace from sources:

.. code-block:: bash

  go install github.com/canonical/workspace/cmd/workspaced

  go install github.com/canonical/workspace/cmd/workspace


Run the daemon
--------------

To run the Workspace daemon, set the `$WORKSPACE` environment variable and use the `workspaced run` sub-command:

.. code-block:: bash

  $ mkdir ~/workspace
  $ export WORKSPACE=~/workspace
  $ workspaced run
  2021-09-15T01:37:23.962Z [workspaced] Started daemon.
  ...

Launch a workspace
------------------

Create a workspace file in a project directory and launch the workspace:

.. code-block:: bash

  $ cat > .workspace.nimble.yaml <<EOF -
  name: nimble
  base: ubuntu@22.04
  sdks:
    go:
      channel: latest/stable
    openjdk:
      channel: latest/stable
  EOF
  $ workspace launch nimble

Development
-----------

Workspace uses a "go test"-compatible `gocheck <https://pkg.go.dev/gopkg.in/check.v1#section-readme>`_:

.. code-block:: bash

  go test ./...
  go test -check.f <TestName|SuiteName>

To run end to end and integration tests:

.. code-block:: bash

  go install github.com/snapcore/spread/cmd/spread@latest
  spread
