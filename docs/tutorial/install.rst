Installation
=======================================

===========
Prerequisites
===========

LXD
~~~~~~~~~~~

Workspace uses `LXD <https://ubuntu.com/lxd>`_ as a container backend. Every workspace is essentially a system container that is created, started, and configured using LXD REST API. Whilst LXD supports a large set of operating systems, Workspace is currently limited to using Ubuntu as a base system for its containers.

-----------
Install LXD on Ubuntu
-----------
There is a chance LXD is already installed on your system, confirm that by
running:

.. code-block:: bash

  snap info lxd

If LXD is found, skip the following steps as Workspace will discover and
configure the required LXD settings automatically. Otherwise, if LXD is not
present on your system, run:

.. code-block:: bash

  sudo snap install lxd
  sudo lxd init --auto

Then ensure that the LXD daemon is active and running:

.. code-block:: bash

  systemctl status snap.lxd.daemon.service

----------------------
Install LXD on other Linux distributives
----------------------

Check `LXD documentation
<https://documentation.ubuntu.com/lxd/en/latest/installing/?_ga=2.224594138.1101634201.1688935617-532732205.1687382301>`_
for the options available for other Linux distributives.


Go
~~~~~~~~~~~~~~~~~~~~~~
The only option to get Workspace that is currently available is installing from
the source code. Therefore, Go run-time must be available to install Workspace.
Run:

----------------------
Install Go on Ubuntu
----------------------

.. code-block:: bash

  sudo snap install --classic --channel=1.20/stable go

----------------------------------------
Install Go on other Linux distributives
----------------------------------------

Check the `official documentation <https://go.dev/doc/install>`_ for the options
available for other Linux distributives.

======================
Install Workspace
======================

Workspace consists of a daemon and a CLI command ``workspace``. Run the
following command to install Workspace for your user:

.. code-block:: bash
  # Install the workspaced
  go install github.com/canonical/workspace/cmd/workspaced

  # Install the CLI
  go install github.com/canonical/workspace/cmd/workspace

======================
Run Workspace
======================
To use the CLI command, the daemon should be up and running in a separate
session:

.. code-block:: bash
  mkdir ~/workspace
  export WORKSPACE=~/workspace
  workspaced run --create-dirs
