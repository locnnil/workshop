.. _ref_workshop_def_yaml:

Workshop definition
===================

Filename convention
-------------------

The name of the workshop definition
file must have the following format: :file:`.workshop.<NAME>.yaml`.

.. tip:: Note the dot at the start.

Here, :samp:`<NAME>` is a placeholder that stands for the actual name
of the workshop itself;
it must start with a lowercase letter
and may include only lowercase letters, digits, hyphens or underscores.


Description
-----------

The definition in the file must be written in
`YAML <https://yaml.org/>`__
and include three required keys:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 1 1 7

   * - Key
     - Value
     - Description

   * - :samp:`name`
     - string
     - Workshop's name, used to reference the workshop itself.

       Must be the same as :samp:`<NAME>`
       in the workshop definition's filename.

   * - :samp:`base`
     - string
     - Workshop's base image
       that provides the underlying OS capabilities.

       It can be :samp:`ubuntu@20.04` or :samp:`ubuntu@22.04`.

   * - :samp:`sdks`
     - object
     - List of individual SDKs
       from the SDK Store to include in the workshop.

       Each entry here points to an existing SDK
       and specifies its retrieval channel.


In turn, any entry in :samp:`sdks` must be named after an existing SDK
that is available from the SDK store.
Each SDK is described with the following keys:

.. list-table::
   :header-rows: 1
   :width: 95
   :widths: 1 1 7

   * - Key
     - Value
     - Description

   * - :samp:`channel` (required)
     - string
     - SDK version to retrieve during
       :ref:`launch <ref_workshop_launch>`
       and
       :ref:`refresh <ref_workshop_refresh>`
       operations.

       It uses a
       `snap-like format <https://snapcraft.io/docs/channels>`__
       of :samp:`<TRACK>/<RISK>`
       without the :samp:`<BRANCH>` part.

   * - :samp:`plugs`
     - object
     - Defines plug bindings;
       each entry must be named after a plug in this SDK
       and contain a single :samp:`bind` key.

       In turn, :samp:`bind` must be a string
       that references a plug of the same interface in a different SDK
       using the :samp:`<SDK>/<PLUG>` format.


JSON Schema
-----------

The following
`JSON Schema <https://json-schema.org/>`__
formalises the description above:

.. literalinclude:: schema.json
   :language: json


Examples
--------

This YAML file defines a :samp:`golang` workshop
with a single :samp:`go` SDK
from the :samp:`latest/stable` channel:

.. code-block:: yaml
   :caption: .workshop.golang.yaml

   name: golang
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/stable


This YAML file defines a :samp:`go-dev` workshop
that uses two SDKs, :samp:`go` and :samp:`dev-tunnel`;
the :samp:`data` plug defined by the :samp:`dev-tunnel` SDK
is bound to the :samp:`mod-cache` plug of the :samp:`go` SDK:

.. code-block:: yaml
   :caption: .workshop.go-dev.yaml

   name: go-dev
   base: ubuntu@22.04
   sdks:
     go:
       channel: latest/candidate
     dev-tunnel:
       channel: latest/edge
       plugs:
         data:
           bind: go:mod-cache


See also
--------

Explanation:

- :ref:`exp_sdk`
- :ref:`exp_base`
- :ref:`exp_workshop_def`


Reference:

- :ref:`ref_workshop_info`
