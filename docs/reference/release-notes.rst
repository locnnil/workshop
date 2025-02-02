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

Currently, |ws_markup| is not yet publicly available,
but you can confidently use the pre-release versions.

Latest version:

- `Workshop v0.1.8 <https://github.com/canonical/workshop/releases/tag/v0.1.8>`_


Older versions:

- `Workshop v0.1.7 <https://github.com/canonical/workshop/releases/tag/v0.1.7>`_
- `Workshop v0.1.6 <https://github.com/canonical/workshop/releases/tag/v0.1.6>`_
- `Workshop v0.1.5 <https://github.com/canonical/workshop/releases/tag/v0.1.5>`_
- `Workshop v0.1.4 <https://github.com/canonical/workshop/releases/tag/v0.1.4>`_
- `Workshop v0.1.3 <https://github.com/canonical/workshop/releases/tag/v0.1.3>`_
- `Workshop v0.1.2 <https://github.com/canonical/workshop/releases/tag/v0.1.2>`_
- `Workshop v0.1.0 <https://github.com/canonical/workshop/releases/tag/v0.1.0>`_


Release policy and schedule
---------------------------

Our release cadence is biweekly, aligned with our development methodology.
Releases follow the `semantic versioning <https://semver.org/>`_ scheme.


.. _release_upgrade:

Upgrade instructions
--------------------

To upgrade, visit our `GitHub page
<https://github.com/canonical/workshop/releases>`_
to download and install the latest snap:

.. code-block:: console

   $ sudo snap install --dangerous --classic ./workshop_0.1.8_amd64.snap

Snaps are available for the :samp:`amd64` and :samp:`arm64` architectures.

For prerequisites and other details, see the `Getting Started
<https://github.com/canonical/workshop?tab=readme-ov-file#getting-started>`_
section on GitHub or follow the :ref:`tutorial`.
