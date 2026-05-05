.. meta::
   :description: Overview of command-line interfaces for Workshop, including
                 user and SDK author tools, with links to detailed command references.

Command-line interfaces
=======================

Of the four command-line interfaces provided by |ws_markup|,
:program:`workshop` and :program:`sdk` are aimed at |ws_markup| users.
Meanwhile, SDK authors who use |sdk_markup|
will primarily interact with :program:`sdkcraft`
for building and publishing SDKs on the host,
and :program:`workshopctl`
for SDK hooks to report state from inside a running workshop:

.. @artefact workshop (CLI)
.. @artefact sdk (CLI)
.. @artefact sdkcraft (CLI)
.. @artefact workshopctl

.. toctree::
   :maxdepth: 1

   sdk
   sdkcraft
   workshop
   workshopctl
