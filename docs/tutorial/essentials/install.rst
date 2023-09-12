Install |project|
=================

Check the prerequisites,
build and install |project|,
then make sure it runs.


Check prerequisites
-------------------

|project| requires
`LXD <https://ubuntu.com/lxd>`_
for low-level operation,
using its
`REST API <https://documentation.ubuntu.com/lxd/en/latest/restapi_landing/>`_
to configure individual *workspaces*.

First, install and
`initialise <https://documentation.ubuntu.com/lxd/en/latest/howto/initialize/>`_
LXD.
It's available as a snap:

.. tabs::
   .. group-tab:: Using ``snap``

      .. code:: shell

         sudo snap install lxd
         sudo lxd init --auto


   .. group-tab:: Other ways

      See the available installation options in
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_.


Next, ensure the
`LXD daemon
<https://documentation.ubuntu.com/lxd/en/latest/explanation/lxd_lxc/#lxd-daemon>`_
is enabled and running:

.. tabs::
   .. group-tab:: Using ``snap``

      .. code:: shell

         sudo snap start --enable lxd.daemon
         snap services lxd.daemon

   .. group-tab:: Other ways

      Refer to
      `LXD documentation
      <https://documentation.ubuntu.com/lxd/en/latest/installing/>`_
      and your distribution's manuals for guidance.


Install |project|
-----------------

Build the ``workspace`` snap
from the |project| source code on
`GitHub
<https://github.com/canonical/workspace>`_:

.. code:: shell

   git clone git@github.com:canonical/workspace.git
   # -- or --
   git clone https://github.com/canonical/workspace.git
   cd workspace
   sudo snap install snapcraft --classic
   snapcraft

Install the resulting ``.snap`` file,
for example:

.. code:: shell

   sudo snap install --devmode ./workspace_0.1.0_amd64.snap


Run |project|
-------------

The snap installs two major components:

- The :program:`workspaced` daemon that exposes a REST API
- The :program:`workspace` CLI tool that uses this API to command |project|

The daemon starts automatically after installation;
the CLI tool is run manually:

.. code:: shell

   workspace --help
