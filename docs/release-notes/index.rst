.. _release_notes:

.. meta::
   :description: Release notes and upgrade instructions for Workshop and
                 SDKcraft, including new features, bug fixes, and
                 version-specific guidance.

Release notes and upgrade instructions
======================================

This section provides an overview of new features, bug fixes,
and backwards-incompatible changes in each version.

Where necessary,
these release notes also include specific upgrade instructions for each version.
For additional guidance, see the
:ref:`general instructions on preparing for and performing an upgrade
<release_upgrade>`.


Releases
--------

.. toctree::
   :hidden:

   Workshop v0.1.30 <v0.1.30>
   Workshop v0.1.29 <v0.1.29>
   Workshop v0.1.28 <v0.1.28>
   Workshop v0.1.27 <v0.1.27>
   Workshop v0.1.26 <v0.1.26>
   Workshop v0.1.25 <v0.1.25>
   Workshop v0.1.24 <v0.1.24>
   Workshop v0.1.23 <v0.1.23>
   Workshop v0.1.22 <v0.1.22>
   Workshop v0.1.21 <v0.1.21>
   Workshop v0.1.20 <v0.1.20>
   Workshop v0.1.19 <v0.1.19>
   Workshop v0.1.18 <v0.1.18>


We provide two binaries: |ws_markup| and |sdk_markup|.

- |ws_markup| (:samp:`amd64` and :samp:`arm64`) is designed for common users.
- |sdk_markup| (:samp:`arm64` only) is intended for SDK publishers.


Currently, neither is publicly available,
but you can confidently use the pre-release versions.


|ws_markup|
~~~~~~~~~~~

Latest version:

- :doc:`Workshop v0.1.30 <v0.1.30>`

Recent versions:

- :doc:`Workshop v0.1.29 <v0.1.29>`
- :doc:`Workshop v0.1.28 <v0.1.28>`
- :doc:`Workshop v0.1.27 <v0.1.27>`
- :doc:`Workshop v0.1.26 <v0.1.26>`
- :doc:`Workshop v0.1.25 <v0.1.25>`
- :doc:`Workshop v0.1.24 <v0.1.24>`
- :doc:`Workshop v0.1.23 <v0.1.23>`
- :doc:`Workshop v0.1.22 <v0.1.22>`
- :doc:`Workshop v0.1.21 <v0.1.21>`
- :doc:`Workshop v0.1.20 <v0.1.20>`
- :doc:`Workshop v0.1.19 <v0.1.19>`
- :doc:`Workshop v0.1.18 <v0.1.18>`


Older versions:


- `Workshop v0.1.17 <https://github.com/canonical/workshop/releases/tag/v0.1.17>`_
- Workshop v0.1.16 didn't go public
- `Workshop v0.1.15 <https://github.com/canonical/workshop/releases/tag/v0.1.15>`_
- `Workshop v0.1.14 <https://github.com/canonical/workshop/releases/tag/v0.1.14>`_
- `Workshop v0.1.13 <https://github.com/canonical/workshop/releases/tag/v0.1.13>`_
- `Workshop v0.1.12 <https://github.com/canonical/workshop/releases/tag/v0.1.12>`_
- `Workshop v0.1.11 <https://github.com/canonical/workshop/releases/tag/v0.1.11>`_
- `Workshop v0.1.10 <https://github.com/canonical/workshop/releases/tag/v0.1.10>`_
- `Workshop v0.1.9 <https://github.com/canonical/workshop/releases/tag/v0.1.9>`_
- `Workshop v0.1.8 <https://github.com/canonical/workshop/releases/tag/v0.1.8>`_
- `Workshop v0.1.7 <https://github.com/canonical/workshop/releases/tag/v0.1.7>`_
- `Workshop v0.1.6 <https://github.com/canonical/workshop/releases/tag/v0.1.6>`_
- `Workshop v0.1.5 <https://github.com/canonical/workshop/releases/tag/v0.1.5>`_
- `Workshop v0.1.4 <https://github.com/canonical/workshop/releases/tag/v0.1.4>`_
- `Workshop v0.1.3 <https://github.com/canonical/workshop/releases/tag/v0.1.3>`_
- `Workshop v0.1.2 <https://github.com/canonical/workshop/releases/tag/v0.1.2>`_
- Workshop v0.1.1 didn't go public
- `Workshop v0.1.0 <https://github.com/canonical/workshop/releases/tag/v0.1.0>`_


|sdk_markup|
~~~~~~~~~~~~

Latest version:

- `SDKcraft 0.1.14 <https://github.com/canonical/sdkcraft/releases/tag/0.1.14>`_

Recent versions:

- `SDKcraft 0.1.13 <https://github.com/canonical/sdkcraft/releases/tag/0.1.13>`_
- `SDKcraft 0.1.12 <https://github.com/canonical/sdkcraft/releases/tag/0.1.12>`_
- `SDKcraft 0.1.11 <https://github.com/canonical/sdkcraft/releases/tag/0.1.11>`_
- SDKcraft v0.1.10 didn't go public; also, version naming scheme dropped the "v"
- `SDKcraft v0.1.9 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.9>`_
- `SDKcraft v0.1.8 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.8>`_
- `SDKcraft v0.1.7 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.7>`_
- `SDKcraft v0.1.6 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.6>`_
- `SDKcraft v0.1.5 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.5>`_
- `SDKcraft v0.1.4 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.4>`_
- `SDKcraft v0.1.3 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.3>`_


Older versions:

- `SDKcraft 0.1.2 <https://github.com/canonical/sdkcraft/releases/tag/0.1.2>`_
- `SDKcraft 0.1.1 <https://github.com/canonical/sdkcraft/releases/tag/0.1.1>`_
- `SDKcraft 0.1.0 <https://github.com/canonical/sdkcraft/releases/tag/0.1.0>`_


Release policy and schedule
---------------------------

Our release cadence is biweekly, aligned with our development methodology.
Releases follow the `semantic versioning <https://semver.org/>`_ scheme.


Long-term support
~~~~~~~~~~~~~~~~~

We only provide support for the latest version of |ws_markup|.
If you encounter issues with an older version,
we recommend upgrading to the latest release first;
see the next section for guidance.


.. _release_upgrade:

Upgrade instructions
--------------------

Authenticate to the Snap Store and refresh the snap
using the `--classic <https://snapcraft.io/docs/install-modes/>`_ option:

.. code-block:: console

   $ sudo snap login
   $ sudo snap refresh --classic workshop


Alternatively, visit our `Releases_` page on GitHub
to download and install the latest snap:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.30_amd64.snap


Snaps are available for the :samp:`amd64` and :samp:`arm64` architectures.

For prerequisites and other details, see the `Getting Started
<https://github.com/canonical/workshop?tab=readme-ov-file#getting-started>`_
section on GitHub, or follow the :ref:`tut_get_started` tutorial section.
