Workspace
=========

**Workspace automates the configuring and management of reproducible development
environments**.

**Use a straightforward YAML to define your development environment**. Workspace
will create a system container, install specified SDKs and packages, and control
its behaviour with life cycle hooks. VS Code, Jupyter Lab and other IDEs can
discover and use your workspace as a work environment. Dispose the environment
when done and keep the host system clean.

**Make the knowledge of your project's dev environments explicit and shared**.
New contributors can start with a single command that launches the required
workspace. It is easier to debug issues in any of the project's supported
environments, perform code reviews or experiment in a separate light-weight
container.

It is common to have a non-trivial project setup with dependencies on particular
Linux distributions, SDKs from multiple publishers, and system and language
packages. Most such projects can organise setup complexity with Workspace.
Examples include AI/ML, Robotics, IoT, EdTech and similar domains.

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

Workspace uses a "go test"-compatible [gocheck](https://pkg.go.dev/gopkg.in/check.v1#section-readme):

.. code-block:: bash

  go test ./...
  go test -check.f <TestName|SuiteName>

To run end to end and integration tests:

.. code-block:: bash

  go install github.com/snapcore/spread/cmd/spread@latest
  spread
