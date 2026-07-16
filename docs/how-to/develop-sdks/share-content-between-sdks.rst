.. _how_share_content_between_sdks:

.. meta::
   :description: How-to guide on sharing a directory between two SDKs in a
                 workshop with the mount interface, covering the providing
                 slot, the consuming plug, and the connections entry that
                 pairs them.

How to share content between SDKs
=================================

.. @tests in tests/docs-how-to/share-content-between-sdks/task.yaml

.. @artefact mount interface
.. @artefact interface plug
.. @artefact interface slot

Two SDKs in the same workshop can share a directory
through the mount interface,
without routing the content through the host.
Three pieces make up the flow:
the providing SDK declares a mount slot,
the consuming SDK declares a mount plug,
and the workshop definition pairs the two
with an explicit :samp:`connections:` entry.

For instance,
the :samp:`uv` and :samp:`jupyter` SDKs ship this pattern out of the box.
:samp:`uv` builds a Python virtual environment
and publishes it through a :samp:`venv` slot;
:samp:`jupyter` consumes that slot through a :samp:`venv` plug,
so JupyterLab runs against the packages :samp:`uv` installed.
Neither SDK names the other.


Prerequisites
-------------

Before starting, ensure you have these requirements satisfied:

- A working |sdk_markup| installation.

- An :file:`sdkcraft.yaml` you can edit
  for each side of the pair.
  If you don't have one yet,
  :ref:`tut_craft_sdks` walks through scaffolding an SDK with
  :command:`sdkcraft init`.


Declare the mount slot
----------------------

The providing SDK exposes a directory with a mount slot.
The required attribute is :samp:`workshop-source`,
which must be an absolute path inside the workshop
and may use :envvar:`$SDK`
to refer to the SDK installation directory:

.. code-block:: yaml
   :caption: sdkcraft.yaml
   :emphasize-lines: 3-6

   # ...

   slots:
     venv:
       interface: mount
       workshop-source: /home/workshop/uv-venv


:samp:`workshop-source` is the only attribute a mount slot accepts.
In particular, a slot cannot mark itself read-only:
:samp:`read-only` is a plug attribute,
so each consumer decides for itself
whether it mounts the shared directory read-only.

.. note::

   A regular SDK cannot expose a directory from the host this way;
   host-rooted mounts are the responsibility of the :samp:`system` SDK.


Declare the mount plug
----------------------

The consuming SDK declares a mount plug
naming the path where the shared directory appears:

.. code-block:: yaml
   :caption: sdkcraft.yaml
   :emphasize-lines: 3-6

   # ...

   plugs:
     venv:
       interface: mount
       workshop-target: $SDK/venv


The plug and the slot don't have to share a name.
:samp:`uv` and :samp:`jupyter` both happen to use :samp:`venv`,
but nothing pairs them by name:
the workshop definition needs to do that,
regardless of the names used.


Connect the SDKs
----------------

Listing both SDKs in a workshop is not enough to pair them.
A mount plug auto-connects only to a slot that the :samp:`system` SDK provides,
never to a slot on another regular SDK.
Left alone, :samp:`jupyter:venv` connects to :samp:`system:mount`
and receives a directory that |ws_markup| allocates on the host,
while :samp:`uv:venv` stays listed but unconnected.

To pair them, name the plug and the slot
in a top-level :samp:`connections:` entry:

.. code-block:: yaml
   :caption: workshop.yaml
   :emphasize-lines: 6-8

   name: dev
   base: ubuntu@24.04
   sdks:
     - name: uv
     - name: jupyter
   connections:
     - plug: jupyter:venv
       slot: uv:venv


Both sides use the :samp:`<SDK-NAME>:<NAME>` form.
Apply the change with :command:`workshop launch`,
or :command:`workshop refresh` for a workshop that is already running:

.. code-block:: console

   $ workshop refresh


.. note::

   A plug named in :samp:`connections:` is claimed by that entry.
   It stops auto-connecting to :samp:`system:mount`,
   so the host directory it used to receive is no longer mounted.
   Removing the entry and refreshing again returns the plug to that default.


Verify the connection
---------------------

Confirm the pairing with :command:`workshop connections`:

.. code-block:: console

   $ workshop connections dev

     INTERFACE  PLUG              SLOT                 NOTES
     mount      dev/jupyter:venv  dev/uv:venv          -
     mount      dev/uv:cache      dev/system:mount     -
     tunnel     -                 dev/jupyter:jupyter  -


On the first row,
the :samp:`SLOT` column names :samp:`dev/uv:venv`
rather than :samp:`dev/system:mount`,
which confirms that :samp:`jupyter` reads from :samp:`uv`
instead of from a host directory.

The second row shows the default the :samp:`connections:` entry overrides:
:samp:`uv:cache` is not named in the workshop definition,
so it auto-connects to :samp:`system:mount`
and receives a host directory.
A dash in the :samp:`PLUG` column marks a slot that nothing consumes,
which is what :samp:`dev/uv:venv` would show
if the :samp:`connections:` entry were missing.


See also
--------

Explanation:

- :ref:`exp_best_dependencies`
- :ref:`exp_mount_interface`
- :ref:`exp_plugs_slots`
- :ref:`exp_system_sdk`
- :ref:`exp_workshop_definition_connections`


How-to guides:

- :ref:`how_add_mounts`
- :ref:`how_declare_plugs_slots`
- :ref:`how_resolve_plug_conflicts`


Reference:

- :ref:`ref_workshop_connections`
- :ref:`ref_workshop_definition`


Tutorial:

- :ref:`tut_craft_sdks`
