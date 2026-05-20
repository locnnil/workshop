.. _exp_sdks:

.. meta::
   :description: Workshop SDK documentation, providing access to
                 explanations of SDK concepts, data storage, lifecycle hooks,
                 and comparisons with traditional container approaches.

SDKs
====

.. @artefact SDK

These topics cover the many aspects of defining and using SDKs
with |ws_markup| and |sdk_markup|.


Understanding SDKs
------------------

SDKs are packages of software dependencies
distributed through the SDK Store or defined locally.
These articles explain what SDKs are,
how they encapsulate functionality,
and how their internal structure is organised:

.. toctree::
   :maxdepth: 1

   concepts
   Parts <parts>


SDK design
----------

When you are creating SDKs, it helps to understand
how they compare to traditional container approaches
and what design patterns lead to maintainable, reusable packages:

.. toctree::
   :maxdepth: 1

   SDKs versus Dockerfiles <sdk-vs-dockerfile>
   Design best practices <best-practices>


Operations and tooling
----------------------

|ws_markup| exposes available SDKs through the :program:`sdk` CLI,
lets SDK authors build and publish them with the :program:`sdkcraft` CLI,
and ships the in-workshop :program:`workshopctl` helper
for SDK hooks to talk back to the daemon:

.. toctree::
   :maxdepth: 1

   sdk-cli
   sdkcraft-cli
   workshopctl-cli
