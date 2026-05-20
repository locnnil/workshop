:hide-toc:

.. _exp_sdkcraft_cli:

.. meta::
   :description: Documentation of the sdkcraft CLI, detailing its role as
                 the SDK-author tool for scaffolding, building, testing, and
                 publishing SDKs to the SDK Store.

sdkcraft (CLI)
==============

.. @artefact sdkcraft (CLI)

|sdk_markup| is the SDK-author tool, shipped as its own snap.
While :command:`workshop` and :command:`sdk` are aimed at workshop users
and :command:`workshopctl` runs inside workshops,
:command:`sdkcraft` is what an SDK publisher uses on their workstation
to scaffold, build, test, try, and publish SDKs to the SDK Store.

There are several categories of commands that vary by their purpose:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 10 11 20

   * - Actions
     - Commands
     - What they do

   * - Lifecycle
     - :command:`build`,
       :command:`clean`,
       :command:`pack`,
       :command:`prime`,
       :command:`pull`,
       :command:`stage`,
       :command:`test`,
       :command:`try`
     - Work through the parts-based build pipeline
       to produce an SDK artefact
       and try it locally before publishing.

   * - Store
     - :command:`create-track`,
       :command:`register`,
       :command:`release`,
       :command:`upload`
     - Claim an SDK name, manage its tracks,
       upload artefacts, and release revisions to channels.

   * - Store account
     - :command:`login`,
       :command:`whoami`
     - Authenticate against the SDK Store
       and inspect the active identity.

   * - Other
     - :command:`init`,
       :command:`version`
     - Bootstrap a new project layout
       and report the installed |sdk_markup| version.


.. note::

   |sdk_markup| is a separate snap, installed independently of |ws_markup|.
   See the `SDKcraft`_ repository for installation and release notes.


See also
--------

Explanation:

- :ref:`exp_sdk_cli`
- :ref:`exp_workshop_cli`


Reference:

- :ref:`ref_cli`


Tutorial:

- :ref:`tut_get_started`
