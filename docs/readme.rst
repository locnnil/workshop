Workshop
========

.. image:: https://readthedocs.com/projects/canonical-workshop/badge/?version=latest&token=a8c81a46da98f75a366a1eef905457dadfa50c23cf3a1c1929a81af05ffea85d
   :target: https://canonical-workshop.readthedocs-hosted.com/en/latest/?badge=latest
   :alt: Documentation Status

**A tool for defining and managing ephemeral development environments**.


Getting Started
---------------

Follow the sections below
or refer to the
`Tutorial
<https://canonical-workshop.readthedocs-hosted.com/en/latest/tutorial/>`_
in our docs for a more detailed introduction to Workshop.

To join the development effort, see `How to contribute <contributing.rst>`_.

To know more about `SDKcraft <https://github.com/canonical/sdkcraft>`_,
the user-facing counterpart to Workshop,
start with its own `Tutorial
<https://canonical-sdkcraft.readthedocs-hosted.com/en/latest/tutorial/>`_.


Installation
~~~~~~~~~~~~

Workshop requires
`LXD 5.21+ <https://canonical.com/lxd>`_
for low-level operation:

.. code-block:: console

   sudo snap install lxd
   sudo lxd init --auto


Build and install the ``workshop`` snap, for example:

.. code-block:: console

   git clone git@github.com:canonical/workshop.git  # or git clone https://github.com/canonical/workshop.git
   cd workshop
   sudo snap install snapcraft --classic
   snapcraft clean && snapcraft
   sudo snap install --dangerous --classic ./workshop_0.1.0_amd64.snap


Launching workshops
-------------------

In the root directory of the project
that you want to use with Workshop,
create a workshop definition file named ``.workshop.<NAME>.yaml``
to list your project's prerequisites,
then run ``workshop launch <NAME>``:

.. code-block:: console

   cat > .workshop.golang.yaml <<EOF -
   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable
   EOF

   workshop launch golang


Workshop downloads and installs the SDKs your definition lists;
the project is now ready to use them.
