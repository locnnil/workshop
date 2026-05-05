.. _ref_index:

.. meta::
   :description: Workshop reference guides, providing technical background
                 for using Workshop and SDKcraft, including command-line interfaces.

Reference
=========

These reference guides
provide technical background
that may be required to use |ws_markup| and |sdk_markup|.


.. _ref_cli:

Command-line interfaces
-----------------------

These articles share the usage details of the command-line tools
provided by |ws_markup| and |sdk_markup|:

.. toctree::
   :titlesonly:
   :class: flat-toctree

   CLI <cli/index>


.. _ref_definitions:

Definition file formats
-----------------------

.. @artefact SDK
.. @artefact workshop (container)

Workshops and SDKs are defined in YAML and share a number of basic elements
such as plugs, base images and so on.
However, both definition types have different purposes and structure:

.. toctree::
   :titlesonly:
   :class: flat-toctree

   Definition files <definition-files/index>


Reference implementations
-------------------------

.. @artefact SDK

This catalogue lists publicly maintained reference SDK and workshop implementations,
grouped by domain:

.. toctree::
   :titlesonly:
   :class: flat-toctree

   reference-implementations


Structure and behavior
----------------------

These topics provide detailed guidance on various aspects
of operating an SDK or a workshop at run-time.
Workshops are largely made of SDKs;
to understand how a workshop runs and the status it has,
it's crucial to know how SDKs are structured and operated:

.. toctree::
   :titlesonly:
   :class: flat-toctree

   sdks
   workshops
   workshop-status

