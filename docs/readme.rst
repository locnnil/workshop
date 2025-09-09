Workshop
========

.. image:: https://readthedocs.com/projects/canonical-workshop/badge/?version=stable&token=a8c81a46da98f75a366a1eef905457dadfa50c23cf3a1c1929a81af05ffea85d
   :target: https://canonical-workshop.readthedocs-hosted.com/stable/?badge=stable
   :alt: Documentation status

**A tool for defining and handling ephemeral development environments**.


Getting Started
---------------

Follow the sections below,
or refer to the
`Tutorial
<https://canonical-workshop.readthedocs-hosted.com/stable/tutorial/>`_
in our docs for a more detailed introduction to Workshop.

To join the development effort, see `How to contribute <contributing.rst>`_.

To know more about `SDKcraft <https://github.com/canonical/sdkcraft/>`_,
the SDK authoring tool for Workshop,
jump straight to the
`SDK crafting guide
<https://canonical-workshop.readthedocs-hosted.com/stable/tutorial/craft-sdks/>`_
in our docs.

Installation
~~~~~~~~~~~~

Authenticate to the Snap Store and install the snap
using the `--classic <https://snapcraft.io/docs/install-modes>`_ option:

.. code-block:: console

   sudo snap login
   sudo snap install --classic workshop


Alternatively, you can download the latest Workshop snap from the
`Releases_` page on GitHub and install it,
using the options
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_,
for example:

.. code-block:: console

   sudo snap install --dangerous --classic ./workshop_0.1.23_amd64.snap


Prerequisites
~~~~~~~~~~~~~

Workshop requires
`LXD 6.3+ <https://canonical.com/lxd>`_
for low-level operation.

If the ``snap install`` command reports an issue with LXD,
install a recent LXD version with ``snap``:

.. code-block:: console

   sudo snap install --channel=6/stable lxd  # to install
   sudo snap refresh --channel=6/stable lxd  # to update


Launching workshops
-------------------

In the directory of the project
that you want to use with Workshop,
create a workshop definition file named ``workshop.yaml``
to list your project's prerequisites,
then run ``workshop launch``:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@22.04
   sdks:
     - name: go
       channel: 22.04/stable


.. code-block:: console

   workshop launch


Workshop downloads and installs the SDKs your definition lists;
the project is now ready to use them.
