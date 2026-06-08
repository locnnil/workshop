.. _release_notes:

.. meta::
   :description: Release notes and upgrade instructions for Workshop and
                 SDKcraft, including new features, bug fixes, and
                 version-specific guidance.

Release notes and upgrade instructions
======================================

Each version brings new features, bug fixes,
and occasionally backwards-incompatible changes.

Where necessary,
these release notes also include specific upgrade instructions for each version.
For additional guidance, see the
:ref:`general instructions on preparing for and performing an upgrade
<release_upgrade>`.


Releases
--------

.. toctree::
   :hidden:

   Workshop v0.9.1 <v0.9.1>
   Workshop v0.9.0 <v0.9.0>


A complete |ws_markup| installation comprises two snaps:

- :program:`workshop` is designed for common users.
- :program:`sdkcraft` is intended for SDK publishers.


Both are available for :samp:`amd64` and :samp:`arm64`.


|ws_markup|
~~~~~~~~~~~

Latest version:

- :doc:`Workshop v0.9.1 <v0.9.1>`


Older versions:

.. note::

   These versions are no longer supported.

- :doc:`Workshop v0.9.0 <v0.9.0>`
- `Workshop v0.1.30 <https://github.com/canonical/workshop/releases/tag/v0.1.30>`_
- `Workshop v0.1.29 <https://github.com/canonical/workshop/releases/tag/v0.1.29>`_
- `Workshop v0.1.28 <https://github.com/canonical/workshop/releases/tag/v0.1.28>`_
- `Workshop v0.1.27 <https://github.com/canonical/workshop/releases/tag/v0.1.27>`_
- `Workshop v0.1.26 <https://github.com/canonical/workshop/releases/tag/v0.1.26>`_
- `Workshop v0.1.25 <https://github.com/canonical/workshop/releases/tag/v0.1.25>`_
- `Workshop v0.1.24 <https://github.com/canonical/workshop/releases/tag/v0.1.24>`_
- `Workshop v0.1.23 <https://github.com/canonical/workshop/releases/tag/v0.1.23>`_
- `Workshop v0.1.22 <https://github.com/canonical/workshop/releases/tag/v0.1.22>`_
- `Workshop v0.1.21 <https://github.com/canonical/workshop/releases/tag/v0.1.21>`_
- `Workshop v0.1.20 <https://github.com/canonical/workshop/releases/tag/v0.1.20>`_
- `Workshop v0.1.19 <https://github.com/canonical/workshop/releases/tag/v0.1.19>`_
- `Workshop v0.1.18 <https://github.com/canonical/workshop/releases/tag/v0.1.18>`_
- `Workshop v0.1.17 <https://github.com/canonical/workshop/releases/tag/v0.1.17>`_
- Workshop v0.1.16 was not released
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
- Workshop v0.1.1 was not released
- `Workshop v0.1.0 <https://github.com/canonical/workshop/releases/tag/v0.1.0>`_


|sdk_markup|
~~~~~~~~~~~~

Latest version:

- `SDKcraft 0.1.14 <https://github.com/canonical/sdkcraft/releases/tag/0.1.14>`_


Older versions:

.. note::

   These versions predate the SDK Store and are no longer supported.

- `SDKcraft 0.1.13 <https://github.com/canonical/sdkcraft/releases/tag/0.1.13>`_
- `SDKcraft 0.1.12 <https://github.com/canonical/sdkcraft/releases/tag/0.1.12>`_
- `SDKcraft 0.1.11 <https://github.com/canonical/sdkcraft/releases/tag/0.1.11>`_
- SDKcraft v0.1.10 was not released; also, version naming scheme dropped the "v"
- `SDKcraft v0.1.9 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.9>`_
- `SDKcraft v0.1.8 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.8>`_
- `SDKcraft v0.1.7 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.7>`_
- `SDKcraft v0.1.6 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.6>`_
- `SDKcraft v0.1.5 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.5>`_
- `SDKcraft v0.1.4 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.4>`_
- `SDKcraft v0.1.3 <https://github.com/canonical/sdkcraft/releases/tag/v0.1.3>`_
- `SDKcraft 0.1.2 <https://github.com/canonical/sdkcraft/releases/tag/0.1.2>`_
- `SDKcraft 0.1.1 <https://github.com/canonical/sdkcraft/releases/tag/0.1.1>`_
- `SDKcraft 0.1.0 <https://github.com/canonical/sdkcraft/releases/tag/0.1.0>`_


Release policy and schedule
---------------------------

Our release cadence is biweekly, aligned with our development methodology.
Releases follow the `semantic versioning <https://semver.org/>`_ scheme.


Long-term support
~~~~~~~~~~~~~~~~~

We only provide support for the latest versions of |ws_markup| and |sdk_markup|.
If you encounter issues with an older version,
we recommend upgrading to the latest release first;
see the next section for guidance.


.. _release_upgrade:

Upgrade instructions
--------------------

Refresh the snaps using the
`--classic <https://snapcraft.io/docs/install-modes/>`_ option:

.. code-block:: console

   $ sudo snap refresh --classic workshop
   $ sudo snap refresh --classic sdkcraft


For prerequisites and other details, see the `Installation
<https://github.com/canonical/workshop?tab=readme-ov-file#installation>`_
section on GitHub, or follow the :ref:`tut_get_started` tutorial section.
