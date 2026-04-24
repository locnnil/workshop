.. _ref_sdk_definition:

.. meta::
   :description: Reference for SDK definition files, including filename conventions,
                 required YAML structure, and field descriptions for Workshop SDKs.

SDK definition
==============

.. @artefact SDK
.. @artefact SDK definition

Filename convention
-------------------

.. @artefact sdkcraft (CLI)

When :ref:`crafting and publishing a regular SDK <tut_craft_sdks>`,
the name of the SDK definition file must be :file:`sdkcraft.yaml` or :file:`.sdkcraft.yaml`.

When an SDK is built from the definition file,
the resulting package contains the SDK metadata in :file:`sdk.yaml`.
The difference is that the :file:`sdkcraft.yaml` file
is used at build time by |sdk_markup|,
while the :file:`sdk.yaml` file
is used at runtime by |ws_markup|.

Accordingly,
in-project SDKs are defined using :file:`sdk.yaml` or :file:`meta/sdk.yaml`
and stored in :file:`.workshop/<NAME>/`.
Because these SDKs are defined in-place rather than built,
they don't support |sdk_markup| build-time features,
like :samp:`build-base`, :samp:`platforms` or :samp:`parts`.

When :ref:`sketching a local SDK <tut_sketch_sdks>`,
the SDK definition file is also named :file:`sdk.yaml`
and stored under :file:`$XDG_DATA_HOME/workshop/`.
|ws_markup| ignores other files in this directory,
but hooks can be defined inline.
Like in-project SDKs,
the sketch SDK doesn't support |sdk_markup| build-time features.


Structure
---------

The definition in the file must be written in
`YAML <https://yaml.org/>`__
and include these top-level fields:
:samp:`name`, :samp:`version`, and :samp:`platforms`.
Other fields are optional.

.. @artefact SDK base image
.. @artefact SDK platforms

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 3 2 15

   * - Key
     - Value
     - Description

   * - :samp:`name`
     - string
     - SDK's name, used to reference it in the workshop definition.


   * - :samp:`base`
     - string
     - SDK's base image
       that provides the underlying OS capabilities.

       It can be :samp:`ubuntu@20.04`, :samp:`ubuntu@22.04`,
       :samp:`ubuntu@24.04`, or :samp:`ubuntu@26.04`.

       SDKs with a :samp:`base` can only be added to a workshop with the same :samp:`base`.
       SDKs without a :samp:`base` can be added to any workshop.

   * - :samp:`build-base`
     - string
     - Base OS used to build the SDK.

       Required by |sdk_markup| if a :samp:`base` is not defined.

   * - :samp:`version`
     - string
     - SDK's arbitrary version;
       semantic versioning is recommended.

       .. note::

          Use quotes to avoid potential data type mismatches:
          without them, :samp:`'1.0'` can be interpreted as a number,
          for example.


   * - :samp:`summary`
     - string
     - A short one-line summary of up to 79 characters.


   * - :samp:`description`
     - string
     - A longer, more detailed description of the SDK, up to one hundred words.


   * - :samp:`license`
     - string
     - Name of the software license under which the SDK is distributed.

       .. note::

          Make sure it matches the individual components of the SDK.


   * - :samp:`platforms`
     - object
     - A collection of named platforms,
       describing where the SDK can be built and installed.

       See :ref:`ref_sdk_platform` for a detailed discussion.


   * - :samp:`parts`
     - object
     - See :ref:`ref_sdk_parts` for a detailed discussion.


   * - :samp:`plugs`
     - object
     - See :ref:`ref_sdk_plugs_slots` for a detailed discussion.

   * - :samp:`slots`
     - object
     - See :ref:`ref_sdk_plugs_slots` for a detailed discussion.


JSON Schema
-----------

The following
`JSON Schema`
formalizes the :file:`sdkcraft.yaml` format:

.. @artefact SDK schema

.. dropdown:: |sdk_markup| definition schema

   .. literalinclude:: schema-sdkcraft.json
      :language: json


This one formalizes the :file:`sdk.yaml` format:

.. dropdown:: SDK definition schema

   .. literalinclude:: schema-sdk.json
      :language: json


Examples
--------

This is a real-world example of an SDK definition file
for a Go development environment.
It involves a non-trivial layout of build and target architectures,
and also uses the :ref:`parts <ref_sdk_parts>` mechanism:

.. literalinclude:: ../../examples/go-sdkcraft.yaml
   :language: yaml
   :caption: sdkcraft.yaml


This YAML file defines an SDK that supports multiple bases:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: multibase
   version: '0.1'
   summary: Multibase SDK
   description: |
     This is my multibase SDK description.
   license: GPL-3.0
   platforms:
     noble:
       build-on: ['ubuntu@24.04:amd64', 'ubuntu@24.04:arm64']
       build-for: 'ubuntu@24.04:all'
     jammy:
       build-on: ['ubuntu@22.04:amd64', 'ubuntu@22.04:arm64']
       build-for: 'ubuntu@22.04:all'


This is a more elaborate example of an SDK
that uses several :ref:`plugs <ref_sdk_plugs_slots>`:

.. code-block:: yaml
   :caption: sdkcraft.yaml

   name: ros2
   title: The ROS 2 SDK
   base: ubuntu@24.04
   version: "0.1"
   summary: The strictly necessary ROS 2 development environment for your project.
   description: |
     The ROS 2 SDK creates a minimum viable development environment
     for your ROS 2 project.
     It sets up a bare-bones ROS 2 workspace
     before installing all of the dependencies
     for the ROS 2 project mounted by workshop.
   
     A developer can thus connect to the workshop
     to immediately build the project.
   license: LGPL-2.1
   platforms:
     amd64:
     arm64:
   
   plugs:
     ros-cache:
       interface: mount
       workshop-target: /home/workshop/.ros
   
     colcon-artifacts:
       interface: mount
       workshop-target: /home/workshop/colcon
   
     gpu:
       interface: gpu


See also
--------

Reference:

- :ref:`ref_sdk_internals`
- :ref:`ref_workshop_definition`

Tutorial:

- :ref:`tut_sketch_sdks`
- :ref:`tut_craft_sdks`
