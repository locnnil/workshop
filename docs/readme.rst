Workshop
========

.. warning::

   Workshop is currently an internal Canonical project.
   All code, documentation, and materials in this repository
   are company-private and must not be shared outside Canonical
   without prior authorization.
   
   Do not redistribute or discuss this content or the project externally
   (at public forums, in social media, with customers)
   until an official public release is announced.


.. image:: https://readthedocs.com/projects/canonical-workshop/badge/?version=stable&token=a8c81a46da98f75a366a1eef905457dadfa50c23cf3a1c1929a81af05ffea85d
   :target: https://canonical-workshop.readthedocs-hosted.com/stable/?badge=stable
   :alt: Documentation status

**A tool for defining and handling ephemeral development environments**.

List your dependencies and components in YAML to define an environment. The key
pieces of a definition are SDKs, independent but connectable units of
functionality created by software publishers and available on the SDK Store.
Workshop simplifies experiments with your environment layout.

It allows you to focus on adding value to your project. With Workshop, you can
launch a setup that previously took hours to configure in a few commands, and be
sure that it stays operational. It assists in issue reproduction, enables
hands-on code reviews, and turns environment updates into manageable
transactions, reducing the need to battle with your tooling every day.


Using Workshop
--------------

In the directory of the project
that you want to use with Workshop,
create a workshop definition file named ``workshop.yaml``
to list your project's prerequisites,
then run ``workshop launch``:

.. code-block:: yaml
   :caption: workshop.yaml

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: go


.. code-block:: console

   workshop launch


Workshop downloads and installs the SDKs your definition lists;
the project is now ready to use them.


Installation
------------

Workshop is supported on Ubuntu and other ``snap``-enabled Linux distributions.

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


Install Workshop
~~~~~~~~~~~~~~~~

Authenticate to the Snap Store and install the snap
using the `--classic <https://snapcraft.io/docs/install-modes>`_ option:

.. code-block:: console

   sudo snap login
   sudo snap install --classic workshop

To get the newest features, install from the edge channel:
``sudo snap install --classic --edge workshop``.

If this command fails, you may need an invitation;
contact Dmitry Lyfar (dmitry.lyfar@canonical.com, @dlyfar on Mattermost).

Alternatively, you can download the latest Workshop snap from the
`Releases <https://github.com/canonical/workshop/releases/>`_
page on GitHub and install it,
using the options
`--dangerous <https://snapcraft.io/docs/install-modes>`_
and
`--classic <https://snapcraft.io/docs/install-modes>`_,
for example:

.. code-block:: console

   sudo snap install --dangerous --classic ./workshop_0.1.29_amd64.snap


The downside of this method is that you will need to manually
check for and install updates.

Documentation
-------------

Refer to the
`Tutorial
<https://canonical-workshop.readthedocs-hosted.com/stable/tutorial/>`_
in our docs for a detailed introduction to Workshop.

To know more about `SDKcraft <https://github.com/canonical/sdkcraft/>`_,
the SDK authoring tool for Workshop,
jump straight to the
`SDK crafting guide
<https://canonical-workshop.readthedocs-hosted.com/stable/tutorial/part-4-craft-sdks/>`_
in our docs.


Community and Support
---------------------

Use the following resources for communication, support, and feedback:

- `Code of conduct <https://ubuntu.com/community/ethos/code-of-conduct>`__

- `Pulse reviews on Discourse <https://discourse.canonical.com/c/engineering/sdk/34>`__

- `Mattermost channel <https://chat.canonical.com/canonical/channels/workshop>`__

- `Product and documentation feedback <https://github.com/canonical/workshop/issues/>`__


Contributions
-------------

To join the development effort, see `How to contribute <contributing.rst>`_.
